package nats

import (
	"log"
	"strings"
	"sync"

	"github.com/meshery/meshkit/broker"
	nats "github.com/nats-io/nats.go"
)

var (
	NewEmptyConnection = &Nats{}
)

// Nats will implement Nats subscribe and publish functionality
type Nats struct {
	nc *nats.Conn
	en broker.Encoder
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

	en, err := broker.NewEncoding(opts.Encoder)
	if err != nil {
		nc.Close()
		// return nil, ErrEncoding(err)
		return nil, nil
	}

	return &Nats{nc: nc, en: en}, nil
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
	em, err := n.en.Encode(message)
	if err != nil {
		// return ErrEncode(err)
		return ErrPublish(err)
	}

	err = n.nc.Publish(subject, em)
	if err != nil {
		return ErrPublish(err)
	}

	return nil
}

// PublishWithChannel - to publish messages with channel
func (n *Nats) PublishWithChannel(subject string, msgch chan *broker.Message) error {
	for msg := range msgch {
		err := n.Publish(subject, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Subscribe - for subscribing messages
// TODO Ques: Do we want to unsubscribe
// TODO will the method-user just subsribe, how will it handle the received messages?
func (n *Nats) Subscribe(subject, queue string, message []byte) error {
	n.wg.Add(1)
	_, err := n.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
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
	_, err := n.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		brokerMsg := &broker.Message{}
		err := n.en.Decode(msg.Data, brokerMsg)
		if err != nil {
			return
		}

		// Send the decoded message to the channel
		msgch <- brokerMsg
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
	if in == nil || *in == *empty {
		return true
	}
	return false
}
