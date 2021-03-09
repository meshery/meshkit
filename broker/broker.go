package broker

var (
	MeshSync    ObjectType = "meshsync"
	SMI         ObjectType = "smi"
	ErrorObject ObjectType = "error"

	Add        EventType = "ADDED"
	Update     EventType = "MODIFIED"
	Delete     EventType = "DELETED"
	ErrorEvent EventType = "ERROR"
)

type ObjectType string
type EventType string

type Message struct {
	ObjectType ObjectType
	EventType  EventType
	Object     interface{}
}

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
}
