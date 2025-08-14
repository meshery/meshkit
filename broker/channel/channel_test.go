package channel

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChannelBrokerHandler(t *testing.T) {
	tests := []struct {
		name           string
		optsSetters    []OptionsSetter
		expectedBuffer uint
		expectedDelay  time.Duration
	}{
		{
			name:           "default options",
			optsSetters:    nil,
			expectedBuffer: 1024,
			expectedDelay:  1 * time.Second,
		},
		{
			name: "custom buffer size",
			optsSetters: []OptionsSetter{
				WithSingleChannelBufferSize(2048),
			},
			expectedBuffer: 2048,
			expectedDelay:  1 * time.Second,
		},
		{
			name: "custom delay",
			optsSetters: []OptionsSetter{
				WithPublishToChannelDelay(2 * time.Second),
			},
			expectedBuffer: 1024,
			expectedDelay:  2 * time.Second,
		},
		{
			name: "custom both",
			optsSetters: []OptionsSetter{
				WithSingleChannelBufferSize(512),
				WithPublishToChannelDelay(500 * time.Millisecond),
			},
			expectedBuffer: 512,
			expectedDelay:  500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewChannelBrokerHandler(tt.optsSetters...)
			require.NotNil(t, handler)
			assert.Equal(t, tt.expectedBuffer, handler.SingleChannelBufferSize)
			assert.Equal(t, tt.expectedDelay, handler.PublishToChannelDelay)
			assert.NotEmpty(t, handler.name)
			assert.NotNil(t, handler.storage)
		})
	}
}

func TestChannelBrokerHandler_IsEmpty(t *testing.T) {
	handler := NewChannelBrokerHandler()
	assert.True(t, handler.IsEmpty())

	// Add a subscription
	msgch := make(chan *broker.Message, 1)
	err := handler.SubscribeWithChannel("test-subject", "test-queue", msgch)
	require.NoError(t, err)
	assert.False(t, handler.IsEmpty())
}

func TestChannelBrokerHandler_Info(t *testing.T) {
	handler := NewChannelBrokerHandler()
	info := handler.Info()
	assert.Contains(t, info, "channel-broker-handler--")
}

func TestChannelBrokerHandler_ConnectedEndpoints(t *testing.T) {
	handler := NewChannelBrokerHandler()

	// Initially empty
	endpoints := handler.ConnectedEndpoints()
	assert.Empty(t, endpoints)

	// Add subscriptions
	msgch1 := make(chan *broker.Message, 1)
	msgch2 := make(chan *broker.Message, 1)

	err := handler.SubscribeWithChannel("subject1", "queue1", msgch1)
	require.NoError(t, err)
	err = handler.SubscribeWithChannel("subject1", "queue2", msgch2)
	require.NoError(t, err)

	endpoints = handler.ConnectedEndpoints()
	assert.Len(t, endpoints, 2)
	assert.Contains(t, endpoints, "subject1::queue1")
	assert.Contains(t, endpoints, "subject1::queue2")
}

func TestChannelBrokerHandler_Publish_NoSubscribers(t *testing.T) {
	handler := NewChannelBrokerHandler()
	message := &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  broker.Add,
		Object:     "test-data",
	}

	// Publish to non-existent subject should not error
	err := handler.Publish("non-existent", message)
	assert.NoError(t, err)
}

func TestChannelBrokerHandler_Publish_WithSubscribers(t *testing.T) {
	handler := NewChannelBrokerHandler()
	message := &broker.Message{
		ObjectType: broker.MeshSync,
		EventType:  broker.Add,
		Object:     "test-data",
	}

	// Create subscribers
	msgch1 := make(chan *broker.Message, 1)
	msgch2 := make(chan *broker.Message, 1)

	err := handler.SubscribeWithChannel("test-subject", "queue1", msgch1)
	require.NoError(t, err)
	err = handler.SubscribeWithChannel("test-subject", "queue2", msgch2)
	require.NoError(t, err)

	// Publish message
	err = handler.Publish("test-subject", message)
	assert.NoError(t, err)

	// Check if messages were received
	select {
	case receivedMsg := <-msgch1:
		assert.Equal(t, message, receivedMsg)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message on queue1")
	}

	select {
	case receivedMsg := <-msgch2:
		assert.Equal(t, message, receivedMsg)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message on queue2")
	}
}

func TestChannelBrokerHandler_PublishWithChannel(t *testing.T) {
	handler := NewChannelBrokerHandler()

	// Create subscriber
	msgch := make(chan *broker.Message, 2)
	err := handler.SubscribeWithChannel("test-subject", "test-queue", msgch)
	require.NoError(t, err)

	// Create publisher channel
	pubCh := make(chan *broker.Message, 2)
	message1 := &broker.Message{ObjectType: broker.MeshSync, EventType: broker.Add, Object: "data1"}
	message2 := &broker.Message{ObjectType: broker.MeshSync, EventType: broker.Update, Object: "data2"}

	// Start publishing
	err = handler.PublishWithChannel("test-subject", pubCh)
	require.NoError(t, err)

	// Send messages
	pubCh <- message1
	pubCh <- message2
	close(pubCh)

	// Check if messages were received
	select {
	case receivedMsg := <-msgch:
		assert.Equal(t, message1, receivedMsg)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for first message")
	}

	select {
	case receivedMsg := <-msgch:
		assert.Equal(t, message2, receivedMsg)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for second message")
	}
}

func TestChannelBrokerHandler_SubscribeWithChannel(t *testing.T) {
	handler := NewChannelBrokerHandler()
	msgch := make(chan *broker.Message, 1)

	// Subscribe
	err := handler.SubscribeWithChannel("test-subject", "test-queue", msgch)
	assert.NoError(t, err)

	// Verify subscription was created
	endpoints := handler.ConnectedEndpoints()
	assert.Contains(t, endpoints, "test-subject::test-queue")
}

func TestChannelBrokerHandler_CloseConnection(t *testing.T) {
	handler := NewChannelBrokerHandler()

	// Add subscriptions
	msgch1 := make(chan *broker.Message, 1)
	msgch2 := make(chan *broker.Message, 1)

	err := handler.SubscribeWithChannel("subject1", "queue1", msgch1)
	require.NoError(t, err)
	err = handler.SubscribeWithChannel("subject2", "queue2", msgch2)
	require.NoError(t, err)

	// Verify subscriptions exist
	assert.False(t, handler.IsEmpty())
	assert.Len(t, handler.ConnectedEndpoints(), 2)

	// Close connection
	handler.CloseConnection()

	// Verify all subscriptions are closed
	assert.True(t, handler.IsEmpty())
	assert.Empty(t, handler.ConnectedEndpoints())
}

func TestChannelBrokerHandler_DeepCopy(t *testing.T) {
	handler := NewChannelBrokerHandler()

	// DeepCopy should return the same handler (not supported)
	copied := handler.DeepCopy()
	assert.Equal(t, handler, copied)
}

func TestChannelBrokerHandler_DeepCopyObject(t *testing.T) {
	handler := NewChannelBrokerHandler()

	// DeepCopyObject should return the same handler (not supported)
	copied := handler.DeepCopyObject()
	assert.Equal(t, handler, copied)
}

func TestChannelBrokerHandler_Subscribe(t *testing.T) {
	handler := NewChannelBrokerHandler()

	// Subscribe method is not supported and should return nil
	err := handler.Subscribe("test-subject", "test-queue", []byte("test"))
	assert.NoError(t, err)
}

func TestChannelBrokerHandler_ThreadSafety(t *testing.T) {
	handler := NewChannelBrokerHandler()

	// Test concurrent subscriptions
	var wg sync.WaitGroup
	numGoroutines := 10

	// Start multiple goroutines that subscribe to different subjects/queues
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subject := fmt.Sprintf("subject-%d", id)
			queue := fmt.Sprintf("queue-%d", id)
			msgch := make(chan *broker.Message, 1)

			err := handler.SubscribeWithChannel(subject, queue, msgch)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all subscriptions were created
	endpoints := handler.ConnectedEndpoints()
	assert.Len(t, endpoints, numGoroutines)

	// Test concurrent publishing
	wg = sync.WaitGroup{}
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subject := fmt.Sprintf("subject-%d", id)
			message := &broker.Message{
				ObjectType: broker.MeshSync,
				EventType:  broker.Add,
				Object:     fmt.Sprintf("data-%d", id),
			}

			err := handler.Publish(subject, message)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Test concurrent reads
	wg = sync.WaitGroup{}
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// These should not cause data races
			handler.IsEmpty()
			handler.ConnectedEndpoints()
			handler.Info()
		}()
	}

	wg.Wait()

	// Test concurrent close
	wg = sync.WaitGroup{}
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.CloseConnection()
		}()
	}

	wg.Wait()

	// Verify everything is cleaned up
	assert.True(t, handler.IsEmpty())
	assert.Empty(t, handler.ConnectedEndpoints())
}

func TestChannelBrokerHandler_Publish_Timeout(t *testing.T) {
	handler := NewChannelBrokerHandler(
		WithPublishToChannelDelay(1*time.Millisecond), // Very short timeout
		WithSingleChannelBufferSize(1),                // Small buffer
	)

	// Subscribe with a channel that will block
	msgch := make(chan *broker.Message, 1)
	err := handler.SubscribeWithChannel("test-subject", "blocking-queue", msgch)
	require.NoError(t, err)

	// Fill the user's channel to block the goroutine
	msgch <- &broker.Message{Object: "blocking-message"}

	// Now publish multiple messages to fill the internal channel
	// The internal channel has buffer size 1, so we need to send more than 1 message
	for i := 0; i < 3; i++ {
		message := &broker.Message{Object: fmt.Sprintf("message-%d", i)}
		err = handler.Publish("test-subject", message)
		if err != nil {
			// We expect an error at some point
			var pubErr *ErrChannelBrokerPublishType
			require.ErrorAs(t, err, &pubErr, "error should be of type ErrChannelBrokerPublishType")
			assert.Empty(t, pubErr.SuccessQueueList)
			assert.Equal(t, []string{"blocking-queue"}, pubErr.FailedQueueList)
			return // Test passed
		}
	}

	// If we get here, the timeout didn't work as expected
	t.Fatal("Expected timeout error but none occurred")
}
