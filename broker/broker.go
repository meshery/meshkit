package broker

import "github.com/nats-io/nats.go"

var (
	NotConnected = "not-connected"
)

type PublishInterface interface {
	Publish(string, *Message) error
	PublishWithChannel(string, chan *Message) error
}

type SubscribeInterface interface {
	Subscribe(string, string, string, []byte) (*nats.Subscription, error)
	SubscribeWithChannel(string, string, string, chan *Message) (*nats.Subscription, error)
}

type ExecInterface interface {
	GetActiveExecSessions() []*string
	GetExecSession(string) *ExecProp
}

type Handler interface {
	PublishInterface
	SubscribeInterface
	ExecInterface
	Info() string
	DeepCopyObject() Handler
	DeepCopyInto(Handler)
	IsEmpty() bool
	Close(string) bool
}
