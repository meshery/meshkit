package nats

import (
	"github.com/meshery/meshkit/errors"
)

const (
	ErrConnectCode        = "meshkit-11118"
	ErrEncodedConnCode    = "meshkit-11119"
	ErrPublishCode        = "meshkit-11120"
	ErrPublishRequestCode = "meshkit-11121"
	ErrQueueSubscribeCode = "meshkit-11122"
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
