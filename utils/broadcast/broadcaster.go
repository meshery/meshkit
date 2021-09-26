/*
Package broadcast implements multi-listener broadcast channels.
See https://github.com/dustin/go-broadcast for original implementation and example.

Package broadcast provides pubsub of messages over channels.
A provider has a Broadcaster into which it Submits messages and into
which subscribers Register to pick up those messages.
*/
package broadcast

import (
	"time"

	"github.com/google/uuid"
)

type BroadcastSource string

const (
	// OperatorSyncChannel is a broadcast channel type for operator status messages.
	OperatorSyncChannel BroadcastSource = "urn:meshery:operator:sync"
)

type BroadcastMessage struct {
	Id     uuid.UUID
	Source BroadcastSource
	Type   string
	Data   interface{}
	Time   time.Time
}

type broadcaster struct {
	input chan BroadcastMessage
	reg   chan chan<- BroadcastMessage
	unreg chan chan<- BroadcastMessage

	outputs map[chan<- BroadcastMessage]bool
}

// The Broadcaster interface describes the main entry points to
// broadcasters.
type Broadcaster interface {
	// Register a new channel to receive broadcasts
	Register(chan<- BroadcastMessage)
	// Unregister a channel so that it no longer receives broadcasts.
	Unregister(chan<- BroadcastMessage)
	// Shut this broadcaster down.
	Close() error
	// Submit a new object to all subscribers
	Submit(BroadcastMessage)
}

func (b *broadcaster) broadcast(m BroadcastMessage) {
	for ch := range b.outputs {
		ch <- m
	}
}

func (b *broadcaster) run() {
	for {
		select {
		case m := <-b.input:
			b.broadcast(m)
		case ch, ok := <-b.reg:
			if ok {
				b.outputs[ch] = true
			} else {
				return
			}
		case ch := <-b.unreg:
			delete(b.outputs, ch)
		}
	}
}

// NewBroadcaster creates a new broadcaster with the given input
// channel buffer length.
func NewBroadcaster(buflen int) Broadcaster {
	b := &broadcaster{
		input:   make(chan BroadcastMessage, buflen),
		reg:     make(chan chan<- BroadcastMessage),
		unreg:   make(chan chan<- BroadcastMessage),
		outputs: make(map[chan<- BroadcastMessage]bool),
	}

	go b.run()

	return b
}

func (b *broadcaster) Register(newch chan<- BroadcastMessage) {
	b.reg <- newch
}

func (b *broadcaster) Unregister(newch chan<- BroadcastMessage) {
	b.unreg <- newch
}

func (b *broadcaster) Close() error {
	close(b.reg)
	return nil
}

func (b *broadcaster) Submit(m BroadcastMessage) {
	if b != nil {
		b.input <- m
	}
}
