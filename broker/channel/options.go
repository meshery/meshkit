package channel

import (
	"time"

	"github.com/meshery/meshkit/logger"
)

type Options struct {
	SingleChannelBufferSize uint
	PublishToChannelDelay   time.Duration
	Logger                  logger.Handler
}

var DefaultOptions = Options{
	SingleChannelBufferSize: 1024,
	PublishToChannelDelay:   1 * time.Second,
	Logger:                  nil, // Will be created in NewChannelBrokerHandler if nil
}

type OptionsSetter func(*Options)

func WithSingleChannelBufferSize(value uint) OptionsSetter {
	return func(o *Options) {
		o.SingleChannelBufferSize = value
	}
}

func WithPublishToChannelDelay(value time.Duration) OptionsSetter {
	return func(o *Options) {
		o.PublishToChannelDelay = value
	}
}

func WithLogger(log logger.Handler) OptionsSetter {
	return func(o *Options) {
		o.Logger = log
	}
}
