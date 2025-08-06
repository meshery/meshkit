package nats

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/meshery/meshkit/broker"
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

// Nats will implement Nats subscribe and publish functionality
type Nats struct {
	nc *nats.Conn
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

	return &Nats{nc: nc, wg: &sync.WaitGroup{}}, nil
}

func (n *Nats) ConnectedEndpoints() (endpoints []string) {
	for _, server := range n.nc.Servers() {
		endpoints = append(endpoints, strings.TrimPrefix(server, "nats://"))
	}
	return
}

func (n *Nats) Info() string {
	if n.nc == nil {
		return broker.NotConnected
	}
	return n.nc.Opts.Name
}

func (n *Nats) CloseConnection() {
	n.nc.Close()
}

// Publish - to publish messages
func (n *Nats) Publish(subject string, message *broker.Message) error {
	data, err := json.Marshal(message)
	if err != nil {
		return ErrPublish(err)
	}
	return n.nc.Publish(subject, data)
}

// PublishWithChannel - publishes all messages from channel
func (n *Nats) PublishWithChannel(subject string, msgch chan *broker.Message) error {
	go func() {
		for msg := range msgch {
			if err := n.Publish(subject, msg); err != nil {
				log.Printf("failed to publish message: %v", err)
			}
		}
	}()
	return nil
}

// Subscribe - for subscribing messages (blocking)
func (n *Nats) Subscribe(subject, queue string, message []byte) error {
	n.wg.Add(1)
	_, err := n.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		copy(message, msg.Data)
		n.wg.Done()
	})
	if err != nil {
		return ErrQueueSubscribe(err)
	}
	n.wg.Wait()
	return nil
}

// SubscribeWithChannel - for subscribing and forwarding to channel
func (n *Nats) SubscribeWithChannel(subject, queue string, msgch chan *broker.Message) error {
	_, err := n.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		var parsed broker.Message
		err := json.Unmarshal(msg.Data, &parsed)
		if err != nil {
			log.Printf("failed to decode message: %v", err)
			return
		}
		msgch <- &parsed
	})
	if err != nil {
		return ErrQueueSubscribe(err)
	}
	return nil
}

func (in *Nats) DeepCopyInto(out broker.Handler) {
	*out.(*Nats) = *in
}

func (in *Nats) DeepCopy() *Nats {
	if in == nil {
		return nil
	}
	out := new(Nats)
	in.DeepCopyInto(out)
	return out
}

func (in *Nats) DeepCopyObject() broker.Handler {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *Nats) IsEmpty() bool {
	empty := &Nats{}
	if in == nil || *in == *empty {
		return true
	}
	return false
}