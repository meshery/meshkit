package kubernetes

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/meshery/meshkit/logger"
)

// PortForwardTarget identifies what to port-forward to. Provide either an
// explicit PodName or a PodLabels selector (a Running pod matching it is chosen,
// and re-resolved on every reconnect so the tunnel follows pod restarts).
type PortForwardTarget struct {
	Namespace  string
	PodName    string
	PodLabels  map[string]string
	RemotePort int32
}

// PortForwarder maintains a self-healing port-forward from a stable local
// address (127.0.0.1:<localPort>) to a pod in the cluster, tunneled through the
// Kubernetes API server exactly like `kubectl port-forward`. The local port is
// fixed for the lifetime of the forwarder so consumers can keep a stable URL; the
// upstream tunnel is re-established on drop/pod-restart until Stop() is called.
//
// It is deliberately target-agnostic so it can back any server-internal TCP
// tunnel (its first consumer is the Meshery Broker / NATS connection for an
// out-of-cluster Meshery).
type PortForwarder struct {
	client *Client
	target PortForwardTarget
	log    logger.Handler

	localPort int

	mu       sync.Mutex
	started  bool
	stopOnce sync.Once
	stopCh   chan struct{}
}

// reconnectBackoff bounds how fast the forwarder retries after a tunnel drop or a
// failed pod resolution.
const portForwardReconnectBackoff = 2 * time.Second

// NewPortForwarder allocates a stable local port and returns a forwarder for the
// target. Call Start to begin forwarding and Stop to tear it down. The logger may
// be nil.
func NewPortForwarder(client *Client, target PortForwardTarget, log logger.Handler) (*PortForwarder, error) {
	if client == nil || client.KubeClient == nil {
		return nil, fmt.Errorf("port-forward: nil kubernetes client")
	}
	if target.RemotePort == 0 {
		return nil, fmt.Errorf("port-forward: RemotePort is required")
	}
	if target.PodName == "" && len(target.PodLabels) == 0 {
		return nil, fmt.Errorf("port-forward: a PodName or PodLabels selector is required")
	}
	localPort, err := freeLocalPort()
	if err != nil {
		return nil, fmt.Errorf("port-forward: could not allocate a local port: %w", err)
	}
	return &PortForwarder{
		client:    client,
		target:    target,
		log:       log,
		localPort: localPort,
		stopCh:    make(chan struct{}),
	}, nil
}

// LocalAddr is the stable local endpoint (127.0.0.1:<port>) consumers connect to.
// It is valid immediately after NewPortForwarder, before the tunnel is up.
func (pf *PortForwarder) LocalAddr() string {
	return net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", pf.localPort))
}

// Start begins forwarding in the background (idempotent). It returns immediately;
// the tunnel is (re)established asynchronously and consumers should tolerate the
// local endpoint being briefly unavailable (the NATS client's retry handles this).
func (pf *PortForwarder) Start() {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if pf.started {
		return
	}
	pf.started = true
	go pf.run()
}

// Stop tears down the forwarder (idempotent).
func (pf *PortForwarder) Stop() {
	pf.stopOnce.Do(func() { close(pf.stopCh) })
}

func (pf *PortForwarder) run() {
	for {
		select {
		case <-pf.stopCh:
			return
		default:
		}
		if err := pf.forwardOnce(); err != nil {
			pf.logf("port-forward to %s/%s:%d dropped: %v", pf.target.Namespace, pf.targetDesc(), pf.target.RemotePort, err)
		}
		select {
		case <-pf.stopCh:
			return
		case <-time.After(portForwardReconnectBackoff):
		}
	}
}

// forwardOnce resolves a pod and blocks in ForwardPorts until the tunnel drops
// (returns an error) or Stop closes stopCh (returns nil).
func (pf *PortForwarder) forwardOnce() error {
	pod, err := pf.resolvePod()
	if err != nil {
		return err
	}

	req := pf.client.KubeClient.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(pf.target.Namespace).
		Name(pod).
		SubResource("portforward")

	cfg := pf.client.RestConfig
	roundTripper, upgrader, err := spdy.RoundTripperFor(&cfg)
	if err != nil {
		return fmt.Errorf("spdy round tripper: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, req.URL())

	ports := []string{fmt.Sprintf("%d:%d", pf.localPort, pf.target.RemotePort)}
	readyCh := make(chan struct{})
	fw, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, ports, pf.stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("new port-forward: %w", err)
	}

	// doneCh releases the logging goroutine when this attempt ends. Without it,
	// a ForwardPorts() that errors before the tunnel is ready would leave the
	// goroutine blocked on readyCh forever, leaking one goroutine per retry.
	doneCh := make(chan struct{})
	defer close(doneCh)
	go func() {
		select {
		case <-readyCh:
			pf.logf("port-forward ready: %s -> %s/%s:%d", pf.LocalAddr(), pf.target.Namespace, pod, pf.target.RemotePort)
		case <-pf.stopCh:
		case <-doneCh:
		}
	}()

	return fw.ForwardPorts()
}

// resolvePod returns the explicit PodName, or a Running pod matching PodLabels.
func (pf *PortForwarder) resolvePod() (string, error) {
	if pf.target.PodName != "" {
		return pf.target.PodName, nil
	}
	selector := labels.SelectorFromSet(pf.target.PodLabels).String()
	pods, err := pf.client.KubeClient.CoreV1().Pods(pf.target.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", fmt.Errorf("list pods (%s): %w", selector, err)
	}
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning && p.DeletionTimestamp == nil {
			return p.Name, nil
		}
	}
	return "", fmt.Errorf("no running pod for selector %q in namespace %q", selector, pf.target.Namespace)
}

func (pf *PortForwarder) targetDesc() string {
	if pf.target.PodName != "" {
		return pf.target.PodName
	}
	return labels.SelectorFromSet(pf.target.PodLabels).String()
}

func (pf *PortForwarder) logf(format string, args ...interface{}) {
	if pf.log != nil {
		pf.log.Info(fmt.Sprintf(format, args...))
	}
}

// freeLocalPort asks the OS for a free TCP port on the loopback interface and
// returns it. There is a small window before the forwarder binds it, which is
// acceptable for this use.
func freeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
