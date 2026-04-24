package events

import (
	"sync"
)

type EventStreamer struct {
	clientChannels []chan interface{}
	clmx           sync.Mutex
}

func NewEventStreamer() *EventStreamer {
	return &EventStreamer{
		clientChannels: make([]chan interface{}, 0),
	}
}

func (e *EventStreamer) Publish(i interface{}) {
	e.clmx.Lock()
	defer e.clmx.Unlock()
	for _, ch := range e.clientChannels {
		go func(ch chan interface{}) {
			ch <- i
		}(ch)
	}
}

func (e *EventStreamer) Subscribe(ch chan interface{}) {
	e.clmx.Lock()
	defer e.clmx.Unlock()
	e.clientChannels = append(e.clientChannels, ch)
}

// Unsubscribe removes ch from the broadcaster's fan-out list so subsequent
// Publish calls do not target it. Safe to call multiple times — a channel
// that is not subscribed is a no-op. Callers must still drain ch if other
// goroutines may already be mid-send when Unsubscribe runs.
func (e *EventStreamer) Unsubscribe(ch chan interface{}) {
	e.clmx.Lock()
	defer e.clmx.Unlock()
	filtered := e.clientChannels[:0]
	for _, c := range e.clientChannels {
		if c != ch {
			filtered = append(filtered, c)
		}
	}
	// Zero out the tail slots so leftover channel references don't pin.
	for i := len(filtered); i < len(e.clientChannels); i++ {
		e.clientChannels[i] = nil
	}
	e.clientChannels = filtered
}
