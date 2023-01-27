package events

import (
	"encoding/json"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
)

type EventStreamer struct {
	clientChannels []chan cloudevents.Event
	clmx           sync.Mutex
}

type Options struct {
	EventData
	Severity
	Category
	Source   string //Source can be a URI of the source that created this event. For eg: In case of adapters, it will be the address of the adapter. For Meshery, it will be "meshery". For any external event, this will be the usual URL of the source.
	TraceID  string //trace ID of the operation which triggered this event. This helps to group all events originating from the same root level operation.
	ParentID string //ID of the operation which triggered this event. This helps to group all events originating from the same operation.
}

type EventData struct {
	Message string `json:"message"`
	Summary string `json:"summary"`
	Details string `json:"details"`
	Error   *Error `json:"error"`
}
type Error struct {
	ProbableCause        string `json:"probableCause"`
	SuggestedRemediation string `json:"suggestedRemediation"`
	ErrorCode            string `json:"errorCode"`
}

func New(op Options) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetTime(time.Now())
	event.SetType(EventPrefix + string(op.Category))
	event.SetSource(op.Source)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetExtension("trace-id", op.TraceID)
	event.SetExtension("parent-id", op.ParentID)
	data, err := json.Marshal(op.EventData)
	if err != nil {
		return nil, err
	}
	err = event.SetData(cloudevents.ApplicationJSON, data)
	return &event, err
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
