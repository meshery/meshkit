package nats

import (
	"time"
)

type Options struct {
	URLS           []string
	ConnectionName string
	Username       string
	Password       string
	ReconnectWait  time.Duration
	MaxReconnect   int
	Encoder        string
}

var DefautOptions = Options{
	URLS:           []string{"not sure"},
	ConnectionName: "default",
	Username:       "user",
	Password:       "pass",
	ReconnectWait:  500 * time.Millisecond,
	MaxReconnect:   3,
}

type OptionsSetter func(*Options)

func WithURLS(value []string) OptionsSetter {
	return func(o *Options) {
		o.URLS = value
	}
}

func WithConnectionName(value string) OptionsSetter {
	return func(o *Options) {
		o.ConnectionName = value
	}
}

func WithUsername(value string) OptionsSetter {
	return func(o *Options) {
		o.Username = value
	}
}

func WithPassword(value string) OptionsSetter {
	return func(o *Options) {
		o.Password = value
	}
}

func WithReconnectWait(value time.Duration) OptionsSetter {
	return func(o *Options) {
		o.ReconnectWait = value
	}
}

func WithMaxReconnect(value int) OptionsSetter {
	return func(o *Options) {
		o.MaxReconnect = value
	}
}
