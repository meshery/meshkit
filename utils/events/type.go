package events

const (
	EventPrefix = "io.meshery."
)

type Category string

const (
	Provisioning Category = "provisioning"
	Registration Category = "registration"
)
