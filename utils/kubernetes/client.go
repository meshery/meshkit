package kubernetes

import (
	"os"
	"path/filepath"

	"github.com/layer5io/meshkit/models"
	"github.com/layer5io/meshkit/utils"
	events "github.com/layer5io/meshkit/utils/events"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"log"
)

// CustomEvent is a custom event type for recording events.
type CustomEvent struct {
	EventType string
	Level     string
	Message   string
}

// DetectKubeConfig detects the kubeconfig for the kubernetes cluster and returns it
func DetectKubeConfig(configfile []byte) (config *rest.Config, err error) {
	if len(configfile) > 0 {
		cfgFile, err := processConfig(configfile)
		if err != nil {
			return nil, err
		}

		models := &models.Kubeconfig{}

		cfgFile, err = processConfig(configfile)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(cfgFile, models); err != nil {
			return nil, err
		}

		for _, clusters := range models.Clusters {
			if config, err = clientcmd.RESTConfigFromKubeConfig(cfgFile); err == nil {
				// Check the `InsecureSkipTLSVerify` field
				if insecureSkipTLSVerify := clusters.Cluster.InsecureSkipTLSVerify; insecureSkipTLSVerify != nil && *insecureSkipTLSVerify {
					// Skip TLS verification if the field is set to true
					config.TLSClientConfig.Insecure = *insecureSkipTLSVerify

					// Create an event to record the insecure connection
					event := CustomEvent{
						EventType: "Warning",
						Level:     "Warning",
						Message:   "Insecure connection to Kubernetes cluster detected",
					}

					log.Println("SKIP TLS verification part was called")

					// Send the event to the client
					handleCustomEvent(event)

				}

				return config, err
			}
		}
	}

	// If deployed within the cluster
	if config, err = rest.InClusterConfig(); err == nil {
		return config, err
	}

	// Look for kubeconfig from the path mentioned in $KUBECONFIGZ
	var models models.Kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		if config, err = clientcmd.BuildConfigFromFlags("", kubeconfig); err == nil {
			for _, cluster := range models.Clusters {
				if cluster.Cluster.InsecureSkipTLSVerify != nil {
					// Skip TLS verification for this cluster
					config.TLSClientConfig.Insecure = true

					// Create an event to record the insecure connection
					event := CustomEvent{
						EventType: "Warning",
						Level:     "Warning",
						Message:   "Insecure connection to Kubernetes cluster detected",
					}
					log.Println("SKIP TLS verification part was called")

					handleCustomEvent(event)
				}
			}

			return config, err
		}
	}

	// Look for kubeconfig at the default path
	path := filepath.Join(utils.GetHome(), ".kube", "config")
	if config, err = clientcmd.BuildConfigFromFlags("", path); err == nil {
		for _, cluster := range models.Clusters {
			if cluster.Cluster.InsecureSkipTLSVerify != nil {
				// Skip TLS verification for this cluster
				config.TLSClientConfig.Insecure = true

				// Create an event to record the insecure connection
				event := CustomEvent{
					EventType: "Warning",
					Level:     "Warning",
					Message:   "Insecure connection to Kubernetes cluster detected",
				}
				log.Println("SKIP TLS verification part was called")

				handleCustomEvent(event)
			}
		}
		return config, err
	}

	return
}

func handleCustomEvent(event CustomEvent) {

	eventStreamer := events.NewEventStreamer()

	clientChannel := make(chan interface{})
	eventStreamer.Subscribe(clientChannel)
	eventStreamer.Publish(event)

	// log.Printf("%s: %s", event.EventType, event.Message)

}
func processConfig(configFile []byte) ([]byte, error) {
	cfg, err := clientcmd.Load(configFile)
	if err != nil {
		return nil, ErrLoadConfig(err)
	}

	err = clientcmdapi.MinifyConfig(cfg)
	if err != nil {
		return nil, ErrValidateConfig(err)
	}

	err = clientcmdapi.FlattenConfig(cfg)
	if err != nil {
		return nil, ErrValidateConfig(err)
	}

	err = clientcmd.Validate(*cfg)
	if err != nil {
		return nil, ErrValidateConfig(err)
	}

	return clientcmd.Write(*cfg)
}
