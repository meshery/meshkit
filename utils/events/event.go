package events

import (
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type EventStreamer struct {
	clientChannels []chan cloudevents.Event
	clmx           sync.Mutex
}

func NewEventStreamer() *EventStreamer {
	return &EventStreamer{
		clientChannels: make([]chan cloudevents.Event, 0),
	}
}

func (e *EventStreamer) Publish(i cloudevents.Event) {
	e.clmx.Lock()
	defer e.clmx.Unlock()
	for _, ch := range e.clientChannels {
		go func(ch chan cloudevents.Event) {
			ch <- i
		}(ch)
	}
}

func (e *EventStreamer) Subscribe(ch chan cloudevents.Event) {
	e.clmx.Lock()
	defer e.clmx.Unlock()
	e.clientChannels = append(e.clientChannels, ch)
}
