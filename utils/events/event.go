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
