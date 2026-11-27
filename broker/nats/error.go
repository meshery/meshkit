package nats

import (
	"github.com/meshery/meshkit/errors"
)

const (
	ErrConnectCode              = "meshkit-11118"
	ErrEncodedConnCode          = "meshkit-11119"
	ErrPublishCode              = "meshkit-11120"
	ErrPublishRequestCode       = "meshkit-11121"
	ErrQueueSubscribeCode       = "meshkit-11122"
	ErrUnsubscribeCode          = "meshkit-11327"
	ErrEnsureStreamCode         = "meshkit-11353"
	ErrPublishDurableCode       = "meshkit-11354"
	ErrJetStreamUnavailableCode = "meshkit-11355"
)

func ErrConnect(err error) error {
	return errors.New(ErrConnectCode, errors.Alert, []string{"Connection to broker failed"}, []string{err.Error()}, []string{"Endpoint might not be reachable"}, []string{"Make sure the NATS endpoint is reachable"})
}
func ErrEncodedConn(err error) error {
	return errors.New(ErrEncodedConnCode, errors.Alert, []string{"Encoding connection failed with broker"}, []string{err.Error()}, []string{"Endpoint might not be reachable"}, []string{"Make sure the NATS endpoint is reachable"})
}
func ErrPublish(err error) error {
	return errors.New(ErrPublishCode, errors.Alert, []string{"Publish failed"}, []string{err.Error()}, []string{"NATS is unhealthy"}, []string{"Make sure NATS is up and running"})
}
func ErrPublishRequest(err error) error {
	return errors.New(ErrPublishRequestCode, errors.Alert, []string{"Publish request failed"}, []string{err.Error()}, []string{"NATS is unhealthy"}, []string{"Make sure NATS is up and running"})
}
func ErrQueueSubscribe(err error) error {
	return errors.New(ErrQueueSubscribeCode, errors.Alert, []string{"Subscription failed"}, []string{err.Error()}, []string{"NATS is unhealthy"}, []string{"Make sure NATS is up and running"})
}
func ErrUnsubscribe(err error) error {
	return errors.New(ErrUnsubscribeCode, errors.Alert, []string{"Unsubscribe failed"}, []string{err.Error()}, []string{"NATS is unhealthy"}, []string{"Make sure NATS is up and running"})
}
func ErrEnsureStream(err error) error {
	return errors.NewV2(ErrEnsureStreamCode, errors.Alert,
		[]string{"Ensure stream failed"},
		[]string{err.Error()},
		[]string{"NATS is unhealthy", "The stream configuration conflicts with an existing stream"},
		[]string{"Make sure NATS is up and running", "Check the existing stream configuration"},
		nil)
}
func ErrPublishDurable(err error) error {
	return errors.NewV2(ErrPublishDurableCode, errors.Alert,
		[]string{"Durable publish failed"},
		[]string{err.Error()},
		[]string{"NATS is unhealthy", "The publish timed out before the stream stored the message"},
		[]string{"Make sure NATS is up and running", "Retry the publish (idempotency key prevents a duplicate)"},
		nil)
}

// errors.None because a broker without JetStream is a normal setup, not a fault
// Callers should fall back to Publish and log the fallback once.
func ErrJetStreamUnavailable(err error) error {
	return errors.NewV2(ErrJetStreamUnavailableCode, errors.None,
		[]string{"JetStream is unavailable"},
		[]string{err.Error()},
		[]string{"JetStream is not enabled on the NATS server", "The stream has not been created yet"},
		[]string{"Enable JetStream on the broker", "Create the stream with EnsureStream before publishing"},
		nil)
}
func IsJetStreamUnavailable(err error) bool {
	return errors.GetCode(err) == ErrJetStreamUnavailableCode
}
