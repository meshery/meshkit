package nats

import (
	"github.com/layer5io/meshkit/errors"
)

const (
	ErrConnectCode        = "11000"
	ErrEncodedConnCode    = "11000"
	ErrPublishCode        = "11001"
	ErrPublishRequestCode = "11001"
	ErrQueueSubscribeCode = "11001"
)

func ErrConnect(err error) error {
	return errors.New(ErrConnectCode, errors.Alert, []string{"Connection to broker failed"}, []string{err.Error()}, []string{}, []string{})
}
func ErrEncodedConn(err error) error {
	return errors.New(ErrEncodedConnCode, errors.Alert, []string{"Encoding connection failed with broker"}, []string{err.Error()}, []string{}, []string{})
}
func ErrPublish(err error) error {
	return errors.New(ErrPublishCode, errors.Alert, []string{"Publish failed"}, []string{err.Error()}, []string{}, []string{})
}
func ErrPublishRequest(err error) error {
	return errors.New(ErrPublishRequestCode, errors.Alert, []string{"Publish request failed"}, []string{err.Error()}, []string{}, []string{})
}
func ErrQueueSubscribe(err error) error {
	return errors.New(ErrQueueSubscribeCode, errors.Alert, []string{"Subscription failed"}, []string{err.Error()}, []string{}, []string{})
}
