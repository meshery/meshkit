package nats

import (
	"github.com/meshery/meshkit/errors"
)

const (
	ErrConnectCode             = "meshkit-11118"
	ErrPublishCode             = "meshkit-11120"
	ErrPublishRequestCode      = "meshkit-11121"
	ErrQueueSubscribeCode      = "meshkit-11122"
	ErrEncodeCode              = "meshkit-11124"
	ErrUnsupportedEncodingCode = "meshkit-11123"
)

func ErrConnect(err error) error {
	return errors.New(ErrConnectCode, errors.Alert, []string{"Connection to broker failed"}, []string{err.Error()}, []string{"Endpoint might not be reachable"}, []string{"Make sure the NATS endpoint is reachable"})
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
func ErrEncode(err error) error {
	return errors.New(ErrEncodeCode, errors.Alert, []string{"Encoding failed"}, []string{err.Error()}, []string{"Data could not be encoded"}, []string{"Make sure the data is serializable"})
}
func ErrUnsupportedEncoding(err error) error {
	return errors.New(ErrUnsupportedEncodingCode, errors.Alert, []string{"Unsupported encoding"}, []string{err.Error()}, []string{"Encoding not supported"}, []string{"Use a supported encoding like JSON or Gob"})
}
