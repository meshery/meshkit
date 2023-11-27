package nats

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrConnectCode        = "11103"
	ErrEncodedConnCode    = "11104"
	ErrPublishCode        = "11105"
	ErrPublishRequestCode = "11101"
	ErrQueueSubscribeCode = "11102"
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
