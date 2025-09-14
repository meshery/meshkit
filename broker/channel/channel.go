package channel

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshkit/utils"
	"github.com/sirupsen/logrus"
)

type ChannelBrokerHandler struct {
	Options
	name string
	// this structure represents [subject] => [queue] => channel
	// so there is a channel per queue per subject
	storage map[string]map[string]chan *broker.Message
	mu      sync.RWMutex // protects storage map from concurrent access
	log     logger.Handler
}

func NewChannelBrokerHandler(optsSetters ...OptionsSetter) *ChannelBrokerHandler {
	options := DefaultOptions
	for _, setOptions := range optsSetters {
		if setOptions != nil {
			setOptions(&options)
		}
	}

	// Use provided logger or create a default one
	log := options.Logger
	if log == nil {
		var err error
		log, err = logger.New("channel-broker", logger.Options{
			Format:   logger.TerminalLogFormat,
			LogLevel: int(logrus.InfoLevel),
		})
		if err != nil {
			// Fallback to a simple logger if creation fails
			log = nil
		}
	}

	return &ChannelBrokerHandler{
		name: fmt.Sprintf(
			"channel-broker-handler--%s",
			uuid.New().String(),
		),
		Options: options,
		storage: make(map[string]map[string]chan *broker.Message),
		log:     log,
	}
}

func (h *ChannelBrokerHandler) ConnectedEndpoints() (endpoints []string) {
	// return subjects::queue list intead of connection endpoints
	h.mu.RLock()
	defer h.mu.RUnlock()

	list := make([]string, 0, len(h.storage))
	for subject, qstorage := range h.storage {
		if qstorage == nil {
			continue
		}
		for queue := range qstorage {
			list = append(
				list,
				fmt.Sprintf(
					"%s::%s",
					subject,
					queue,
				),
			)
		}

	}
	return list
}

func (h *ChannelBrokerHandler) Info() string {
	// return name because nats implementation returns name
	return h.name
}

func (h *ChannelBrokerHandler) CloseConnection() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for subject, qstorage := range h.storage {
		for queue, ch := range qstorage {
			if !utils.IsClosed(ch) {
				close(ch)
			}
			delete(qstorage, queue)
		}
		delete(h.storage, subject)
	}
}

// Publish - to publish messages
func (h *ChannelBrokerHandler) Publish(subject string, message *broker.Message) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.storage[subject]) <= 0 {
		// nobody is listening => not publishing
		return nil
	}

	var successList []string
	var failedList []string

	for queue, ch := range h.storage[subject] {
		select {
		case ch <- message:
			successList = append(successList, queue)
		case <-time.After(h.PublishToChannelDelay):
			failedList = append(failedList, queue)
		}
	}

	if len(failedList) > 0 {
		return NewErrChannelBrokerPublish(
			fmt.Errorf("failed to publish to one or more queue for subject %s", subject),
			successList,
			failedList,
		)
	}

	return nil
}

// PublishWithChannel - to publish messages with channel
func (h *ChannelBrokerHandler) PublishWithChannel(subject string, msgch chan *broker.Message) error {
	go func() {
		// as soon as this channel will be closed, for loop will end
		for msg := range msgch {
			if err := h.Publish(subject, msg); err != nil {
				// Use proper logger if available, otherwise fallback to fmt.Printf
				if h.log != nil {
					h.log.Error(err)
				} else {
					fmt.Printf("Error publishing message to subject %s: %v\n", subject, err)
				}
			}
		}
	}()
	return nil
}

// Subscribe - for subscribing messages
func (h *ChannelBrokerHandler) Subscribe(subject, queue string, message []byte) error {
	// Looks like current version with nats does not seem to be correct
	// it adds callback which is executed on every message
	// and if queue is populated with messages it keeps waiting, in the end it returns the last one;
	// it does not unsubscribe and callback will keep executing and write to local variable, which probably will cause panic;

	// Not supported

	return nil
}

// SubscribeWithChannel will publish all the messages received to the given channel
func (h *ChannelBrokerHandler) SubscribeWithChannel(subject, queue string, msgch chan *broker.Message) error {
	h.mu.Lock()

	if h.storage[subject] == nil {
		h.storage[subject] = make(map[string]chan *broker.Message)
	}

	if h.storage[subject][queue] == nil {
		h.storage[subject][queue] = make(chan *broker.Message, h.SingleChannelBufferSize)
	}
	// a local copy of the channel before starting the goroutine. That way the goroutine never touches the shared map directly
	ch := h.storage[subject][queue]
	h.mu.Unlock()

	go func(c chan *broker.Message) {
		for message := range c {
			// this flow is correct as if we have more than one consumer for one queue
			// only one will receive the message
			msgch <- message
		}
	}(ch)

	return nil
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (h *ChannelBrokerHandler) DeepCopyInto(out broker.Handler) {
	// Not supported: deep copy is not implemented for ChannelBrokerHandler
	// This method is required by the broker.Handler interface but not used in practice
	// Any modification to the "copied" object will affect the original
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new ChannelBrokerHandler.
func (h *ChannelBrokerHandler) DeepCopy() broker.Handler {
	// Not supported: returns the original instance, not a copy
	// This method is required by the broker.Handler interface but not used in practice
	// Any modification to the "copied" object will affect the original
	return h
}

// DeepCopyObject is a deepcopy function, copying the receiver, creating a new broker.Handler.
func (h *ChannelBrokerHandler) DeepCopyObject() broker.Handler {
	// Not supported: returns the original instance, not a copy
	// This method is required by the broker.Handler interface but not used in practice
	// Any modification to the "copied" object will affect the original
	return h
}

// Check if the connection object is empty
func (h *ChannelBrokerHandler) IsEmpty() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.storage) <= 0
}
