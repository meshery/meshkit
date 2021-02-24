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
	return errors.NewDefault(ErrConnectCode, "Connection to broker failed", err.Error())
}
func ErrEncodedConn(err error) error {
	return errors.NewDefault(ErrEncodedConnCode, "Encoding connection failed with broker", err.Error())
}
func ErrPublish(err error) error {
	return errors.NewDefault(ErrPublishCode, "Publish failed", err.Error())
}
func ErrPublishRequest(err error) error {
	return errors.NewDefault(ErrPublishRequestCode, "Publish request failed", err.Error())
}
func ErrQueueSubscribe(err error) error {
	return errors.NewDefault(ErrQueueSubscribeCode, "Subscription failed", err.Error())
}
