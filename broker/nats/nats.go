package nats

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/layer5io/meshkit/broker"
	nats "github.com/nats-io/nats.go"
)

var (
	NewEmptyConnection = &Nats{}
	activeExecSessions = make(map[string]broker.ExecProp)
)

type Options struct {
	URLS           []string
	ConnectionName string
	Username       string
	Password       string
	ReconnectWait  time.Duration
	MaxReconnect   int
}

// Nats will implement Nats subscribe and publish functionality
type Nats struct {
	ec *nats.EncodedConn
	wg *sync.WaitGroup
}

// New - constructor
func New(opts Options) (broker.Handler, error) {
	nc, err := nats.Connect(strings.Join(opts.URLS, ","),
		nats.Name(opts.ConnectionName),
		nats.ReconnectWait(opts.ReconnectWait),
		nats.MaxReconnects(opts.MaxReconnect),
		nats.UserInfo(opts.Username, opts.Password),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("client disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Printf("client reconnected")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Printf("client closed")
		}),
		nats.DiscoveredServersHandler(func(nc *nats.Conn) {
			log.Printf("Known servers: %v\n", nc.Servers())
			log.Printf("Discovered servers: %v\n", nc.DiscoveredServers())
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			log.Printf("Error: %v", err)
		}),
	)
	if err != nil {
		return nil, ErrConnect(err)
	}

	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, ErrEncodedConn(err)
	}

	return &Nats{ec: ec}, nil
}

func (n *Nats) Info() string {
	if n.ec == nil || n.ec.Conn == nil {
		return broker.NotConnected
	}
	return n.ec.Conn.Opts.Name
}

// Publish - to publish messages
func (n *Nats) Publish(subject string, message *broker.Message) error {
	err := n.ec.Publish(subject, message)
	if err != nil {
		return ErrPublish(err)
	}
	return nil
}

// PublishWithChannel - to publish messages with channel
func (n *Nats) PublishWithChannel(subject string, msgch chan *broker.Message) error {
	err := n.ec.BindSendChan(subject, msgch)
	if err != nil {
		return ErrPublish(err)
	}
	return nil
}

// Subscribe - for subscribing messages
// TODO Ques: Do we want to unsubscribe
// TODO will the method-user just subsribe, how will it handle the received messages?
func (n *Nats) Subscribe(reqID, subject, queue string, message []byte) (*nats.Subscription, error) {
	n.wg.Add(1)
	sub, err := n.ec.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		message = msg.Data
		n.wg.Done()
	})
	if err != nil {
		return nil, ErrQueueSubscribe(err)
	}
	activeExecSessions[reqID] = broker.ExecProp{
		ID:             subject,
		ReceiveChannel: make(chan *broker.Message),
		Subscription:   sub,
	}
	n.wg.Wait()

	return sub, nil
}

// SubscribeWithChannel will publish all the messages received to the given channel
func (n *Nats) SubscribeWithChannel(reqID, subject, queue string, msgch chan *broker.Message) (*nats.Subscription, error) {
	sub, err := n.ec.BindRecvQueueChan(subject, queue, msgch)
	if err != nil {
		return nil, ErrQueueSubscribe(err)
	}
	activeExecSessions[reqID] = broker.ExecProp{
		ID:             subject,
		ReceiveChannel: make(chan *broker.Message),
		Subscription:   sub,
	}

	return sub, nil
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Nats) DeepCopyInto(out broker.Handler) {
	*out.(*Nats) = *in
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new Nats.
func (in *Nats) DeepCopy() *Nats {
	if in == nil {
		return nil
	}
	out := new(Nats)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is a deepcopy function, copying the receiver, creating a new broker.Handler.
func (in *Nats) DeepCopyObject() broker.Handler {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// Check if the connection object is empty
func (in *Nats) IsEmpty() bool {
	empty := &Nats{}
	if in == nil || *in == *empty {
		return true
	}
	return false
}

// Get active exec sessions from NATS
func (in *Nats) GetActiveExecSessions() []*string {
	sessions := make([]*string, 0)
	for _, s := range activeExecSessions {
		sessions = append(sessions, &s.ID)
	}
	return sessions
}

// Get an exec session from NATS
func (in *Nats) GetExecSession(reqID string) *broker.ExecProp {
	if ses, ok := activeExecSessions[reqID]; ok {
		return &ses
	}
	return nil
}

// Close exec session from NATS
func (in *Nats) Close(reqID string) bool {
	if ses, ok := activeExecSessions[reqID]; ok {
		if err := ses.Subscription.Unsubscribe(); err != nil {
			return false
		}

		delete(activeExecSessions, reqID)

		return true
	}

	return false
}
