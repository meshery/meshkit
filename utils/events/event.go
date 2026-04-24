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

// Unsubscribe removes every occurrence of ch from the broadcaster's fan-out
// list so subsequent Publish calls do not target it. Subscribe does not
// deduplicate, so a channel that was subscribed N times is fully detached
// after a single Unsubscribe — callers that want "unsubscribe exactly one
// logical subscription" must manage that counting themselves.
//
// Unsubscribe is safe to call multiple times; a channel that is not
// subscribed is a no-op. It does not, however, wait for sender goroutines
// already launched by an earlier Publish: those goroutines hold ch by
// reference and will proceed to `ch <- i`. Callers must therefore:
//   - drain ch (or keep a reader around) if an in-flight send could block,
//     and
//   - NOT close ch immediately after Unsubscribe returns, since a racing
//     Publish sender would panic with send-on-closed-channel.
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
