package broker

var (
	NotConnected = "not-connected"
)

type PublishInterface interface {
	Publish(string, *Message) error
	PublishWithChannel(string, chan *Message) error
}

type SubscribeInterface interface {
	Subscribe(string, string, []byte) error
	SubscribeWithChannel(string, string, chan *Message) error
}

type Handler interface {
	PublishInterface
	SubscribeInterface
	Info() string
	DeepCopyObject() Handler
	DeepCopyInto(Handler)
	IsEmpty() bool
	CloseConnection()
	ConnectedEndpoints() []string //To get the IP addresses of connected endpoints
}
