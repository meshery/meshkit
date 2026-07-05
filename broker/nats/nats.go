package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/logger"
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
	Logger         logger.Handler
}

// subscriptions tracks the live nats.Subscription handles per subject so they
// can be torn down by Unsubscribe. It is held behind a pointer on Nats so the
// shallow DeepCopyInto copy shares it (and does not copy the mutex by value).
type subscriptions struct {
	mu    sync.Mutex
	items map[string][]*nats.Subscription
}

func newSubscriptions() *subscriptions {
	return &subscriptions{items: make(map[string][]*nats.Subscription)}
}

// add records a subscription for a subject.
func (s *subscriptions) add(subject string, sub *nats.Subscription) {
	if s == nil || sub == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[subject] = append(s.items[subject], sub)
}

// take removes and returns all subscriptions recorded for a subject.
func (s *subscriptions) take(subject string) []*nats.Subscription {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	subs := s.items[subject]
	delete(s.items, subject)
	return subs
}

// Nats will implement Nats subscribe and publish functionality
type Nats struct {
	nc     *nats.Conn
	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	log    logger.Handler
	subs   *subscriptions
}

// New - constructor
func New(opts Options) (broker.Handler, error) {
	nc, err := nats.Connect(strings.Join(opts.URLS, ","),
		nats.Name(opts.ConnectionName),
		nats.ReconnectWait(opts.ReconnectWait),
		nats.MaxReconnects(opts.MaxReconnect),
		nats.UserInfo(opts.Username, opts.Password),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if opts.Logger != nil {
				opts.Logger.Error(err)
			} else {
				log.Printf("client disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			if opts.Logger != nil {
				opts.Logger.Info("client reconnected")
			} else {
				log.Printf("client reconnected")
			}
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			if opts.Logger != nil {
				opts.Logger.Info("client closed")
			} else {
				log.Printf("client closed")
			}
		}),
		nats.DiscoveredServersHandler(func(nc *nats.Conn) {
			msg := fmt.Sprintf("Known servers: %v", nc.Servers())
			if opts.Logger != nil {
				opts.Logger.Info(msg)
			} else {
				log.Printf("%s", msg)
			}
			msg2 := fmt.Sprintf("Discovered servers: %v", nc.DiscoveredServers())
			if opts.Logger != nil {
				opts.Logger.Info(msg2)
			} else {
				log.Printf("%s", msg2)
			}
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			if opts.Logger != nil {
				opts.Logger.Error(err)
			} else {
				log.Printf("Error: %v", err)
			}
		}),
	)
	if err != nil {
		return nil, ErrConnect(err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Use provided logger or create a default one
	lg := opts.Logger
	if lg == nil {
		var lerr error
		lg, lerr = logger.New("nats-handler", logger.Options{
			Format:   logger.TerminalLogFormat,
			LogLevel: 4, // Info
		})
		if lerr != nil {
			// fallback to nil; we'll use std log where necessary
			lg = nil
		}
	}

	return &Nats{nc: nc, wg: &sync.WaitGroup{}, ctx: ctx, cancel: cancel, log: lg, subs: newSubscriptions()}, nil
}

func (n *Nats) ConnectedEndpoints() (endpoints []string) {
	if n == nil || n.nc == nil {
		return
	}
	for _, server := range n.nc.Servers() {
		endpoints = append(endpoints, strings.TrimPrefix(server, "nats://"))
	}
	return
}

func (n *Nats) Info() string {
	if n == nil || n.nc == nil {
		return broker.NotConnected
	}
	return n.nc.Opts.Name
}

func (n *Nats) CloseConnection() {
	if n == nil {
		return
	}
	// Cancel background go routines first
	if n.cancel != nil {
		n.cancel()
	}
	if n.nc != nil {
		n.nc.Close()
	}
}

// Publish - to publish messages (uses JSON encoding)
func (n *Nats) Publish(subject string, message *broker.Message) error {
	if n == nil || n.nc == nil {
		return ErrPublish(fmt.Errorf("nats connection is not initialized"))
	}

	data, err := json.Marshal(message)
	if err != nil {
		if n.log != nil {
			n.log.Error(err)
		} else {
			log.Printf("failed to marshal message: %v", err)
		}
		return ErrPublish(err)
	}

	if err := n.nc.Publish(subject, data); err != nil {
		if n.log != nil {
			n.log.Error(err)
		} else {
			log.Printf("failed to publish: %v", err)
		}
		return ErrPublish(err)
	}
	return nil
}

// PublishWithChannel - publishes all messages from channel
func (n *Nats) PublishWithChannel(subject string, msgch chan *broker.Message) error {
	if n == nil {
		return ErrPublish(fmt.Errorf("nats handler is nil"))
	}
	go func() {
		for {
			select {
			case <-n.ctx.Done():
				return
			case msg, ok := <-msgch:
				if !ok {
					return
				}
				if err := n.Publish(subject, msg); err != nil {
					if n.log != nil {
						n.log.Error(err)
					} else {
						log.Printf("failed to publish message: %v", err)
					}
				}
			}
		}
	}()
	return nil
}

// Subscribe - for subscribing messages (blocking)
func (n *Nats) Subscribe(subject, queue string, message []byte) error {
	if n == nil || n.nc == nil {
		return ErrQueueSubscribe(fmt.Errorf("nats connection is not initialized"))
	}

	n.wg.Add(1)
	sub, err := n.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		copied := copy(message, msg.Data)
		if copied < len(msg.Data) {
			if n.log != nil {
				n.log.Info(fmt.Sprintf("warning: message truncated in Subscribe. buffer size: %d, message size: %d", len(message), len(msg.Data)))
			} else {
				log.Printf("warning: message truncated in Subscribe. buffer size: %d, message size: %d", len(message), len(msg.Data))
			}
		}
		n.wg.Done()
	})
	if err != nil {
		if n.log != nil {
			n.log.Error(err)
		} else {
			log.Printf("queue subscribe error: %v", err)
		}
		return ErrQueueSubscribe(err)
	}
	n.subs.add(subject, sub)
	n.wg.Wait()
	return nil
}

// SubscribeWithChannel - for subscribing and forwarding to channel (decodes JSON)
func (n *Nats) SubscribeWithChannel(subject, queue string, msgch chan *broker.Message) error {
	if n == nil || n.nc == nil {
		return ErrQueueSubscribe(fmt.Errorf("nats connection is not initialized"))
	}

	sub, err := n.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		var parsed broker.Message
		if err := json.Unmarshal(msg.Data, &parsed); err != nil {
			if n.log != nil {
				n.log.Error(err)
			} else {
				log.Printf("failed to decode message: %v", err)
			}
			return
		}
		msgch <- &parsed
	})
	if err != nil {
		if n.log != nil {
			n.log.Error(err)
		} else {
			log.Printf("queue subscribe error: %v", err)
		}
		return ErrQueueSubscribe(err)
	}
	n.subs.add(subject, sub)
	return nil
}

// Unsubscribe tears down every subscription previously created for the subject
// and removes them from tracking. A nil/uninitialized connection has no
// subscriptions, so it is a no-op; it is also safe to call more than once.
func (n *Nats) Unsubscribe(subject string) error {
	if n == nil || n.nc == nil {
		return nil
	}

	subs := n.subs.take(subject)
	var errs []error
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		if err := sub.Unsubscribe(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		err := ErrUnsubscribe(errors.Join(errs...))
		if n.log != nil {
			n.log.Error(err)
		} else {
			log.Printf("unsubscribe error: %v", err)
		}
		return err
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
	if in == nil {
		return true
	}
	return in.nc == nil && in.wg == nil && in.ctx == nil && in.cancel == nil
}