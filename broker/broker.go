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
	// Unsubscribe tears down every subscription previously created for the given
	// subject (across all queue groups) and releases the resources associated
	// with them - for the NATS handler the underlying nats.Subscription(s), for
	// the channel handler the per-queue delivery channels. It is safe to call for
	// a subject with no active subscriptions (a no-op) and safe to call more than
	// once. Callers that create a subscription for the lifetime of a session
	// (e.g. an interactive exec/log stream keyed by a unique subject) should call
	// Unsubscribe on teardown so the subscription and its delivery goroutine do
	// not leak.
	Unsubscribe(subject string) error
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
	IsConnected() bool            //Whether the underlying connection is currently live
}
