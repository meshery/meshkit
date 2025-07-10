package nats

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/errors"
	nats "github.com/nats-io/nats.go"
)

var (
	NewEmptyConnection = &Nats{}
)

type Options struct {
	URLS           []string
	ConnectionName string
	Username       string
	Password       string
	ReconnectWait  time.Duration
	MaxReconnect   int
}

// NatsConn defines the minimal interface for a NATS connection used by Nats
// Only the methods used in Nats are included
type NatsConn interface {
	Servers() []string
	Drain() error
	Close()
	Publish(subject string, data []byte) error
	QueueSubscribe(subject, queue string, cb func(msg *nats.Msg)) (*nats.Subscription, error)
	// Opts returns the options struct (for Info)
	Opts() nats.Options
}

// natsConnWrapper adapts *nats.Conn to the NatsConn interface
type natsConnWrapper struct {
	*nats.Conn
}

func (w *natsConnWrapper) Opts() nats.Options {
	return w.Conn.Opts
}

func (w *natsConnWrapper) QueueSubscribe(subject, queue string, cb func(msg *nats.Msg)) (*nats.Subscription, error) {
	// Adapt the callback to nats.MsgHandler
	return w.Conn.QueueSubscribe(subject, queue, nats.MsgHandler(cb))
}

// Nats will implement Nats subscribe and publish functionality
type Nats struct {
	conn NatsConn
	wg   *sync.WaitGroup
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

	return &Nats{conn: &natsConnWrapper{nc}, wg: &sync.WaitGroup{}}, nil
}

func (n *Nats) ConnectedEndpoints() (endpoints []string) {
	for _, server := range n.conn.Servers() {
		endpoints = append(endpoints, strings.TrimPrefix(server, "nats://"))
	}
	return
}

func (n *Nats) Info() string {
	if n.conn == nil {
		return broker.NotConnected
	}
	return n.conn.Opts().Name
}

func (n *Nats) CloseConnection() {
	if n.conn != nil {
		if err := n.conn.Drain(); err != nil {
			log.Printf("nats: drain error: %v", err)
		}
		n.conn.Close()
	}
}

// Publish - to publish messages
func (n *Nats) Publish(subject string, message *broker.Message) error {
	if message == nil {
		return ErrPublish(errors.New(
			"nats_publish_error",
			errors.Alert,
			[]string{"message is nil"},
			[]string{},
			[]string{},
			[]string{},
		))
	}
	b, err := json.Marshal(message)
	if err != nil {
		return ErrPublish(err)
	}
	return n.conn.Publish(subject, b)
}

// PublishWithChannel - to publish messages with channel
func (n *Nats) PublishWithChannel(subject string, msgch chan *broker.Message) error {
	go func() {
		for msg := range msgch {
			b, err := json.Marshal(msg)
			if err != nil {
				log.Printf("nats: JSON marshal error: %v", err)
				continue
			}
			if err := n.conn.Publish(subject, b); err != nil {
				log.Printf("nats: publish error for subject %s: %v", subject, err)
			}
		}
	}()
	return nil
}

// Subscribe - for subscribing messages
// TODO Ques: Do we want to unsubscribe
// TODO will the method-user just subsribe, how will it handle the received messages?
func (n *Nats) Subscribe(subject, queue string, message []byte) error {
	n.wg.Add(1)
	_, err := n.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		message = msg.Data
		n.wg.Done()
	})
	if err != nil {
		return ErrQueueSubscribe(err)
	}
	n.wg.Wait()

	return nil
}

// SubscribeWithChannel will publish all the messages received to the given channel
func (n *Nats) SubscribeWithChannel(subject, queue string, msgch chan *broker.Message) error {
	_, err := n.conn.QueueSubscribe(subject, queue, func(m *nats.Msg) {
		var msg broker.Message
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			log.Printf("nats: unable to unmarshal message: %v", err)
		} else {
			msgch <- &msg
		}
	})
	if err != nil {
		return ErrQueueSubscribe(err)
	}

	return nil
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
	return in == nil || *in == *empty
}
