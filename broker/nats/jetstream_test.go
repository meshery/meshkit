package nats

import (
	"context"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/meshery/meshkit/errors"
	"github.com/nats-io/nats.go/jetstream"
)

// newTestHandler connects a Nats handler to a fresh embedded server.
func newTestHandler(t *testing.T, enableJetStream bool) *Nats {
	t.Helper()
	natsServer := runServer(t, enableJetStream)
	handler, err := New(Options{URLS: []string{natsServer.ClientURL()}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(handler.CloseConnection)
	return handler.(*Nats)
}

var testStream = JetStreamOptions{
	StreamName: "TEST_EVENTS",
	Subjects:   []string{"test.events"},
}

func TestEnsureStreamIsIdempotent(t *testing.T) {
	natsHandler := newTestHandler(t, true)
	for call := 1; call <= 2; call++ {
		if err := natsHandler.EnsureStream(context.Background(), testStream); err != nil {
			t.Fatalf("EnsureStream call %d: %v", call, err)
		}
	}
}

func TestEnsureStreamCapsDuplicatesAtMaxAge(t *testing.T) {
	testCases := []struct {
		name       string
		duplicates time.Duration
	}{
		{name: "default window", duplicates: 0},
		{name: "explicit window above MaxAge", duplicates: 2 * time.Minute},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			natsHandler := newTestHandler(t, true)
			options := testStream
			options.MaxAge = 30 * time.Second
			options.Duplicates = testCase.duplicates
			if err := natsHandler.EnsureStream(context.Background(), options); err != nil {
				t.Fatalf("EnsureStream: %v", err)
			}
			jetStreamClient, err := jetstream.New(natsHandler.nc)
			if err != nil {
				t.Fatalf("jetstream.New: %v", err)
			}
			stream, err := jetStreamClient.Stream(context.Background(), options.StreamName)
			if err != nil {
				t.Fatalf("get stream: %v", err)
			}
			if got := stream.CachedInfo().Config.Duplicates; got != 30*time.Second {
				t.Fatalf("Duplicates = %v, want 30s", got)
			}
		})
	}
}

func TestEnsureStreamWithoutJetStream(t *testing.T) {
	natsHandler := newTestHandler(t, false)
	err := natsHandler.EnsureStream(context.Background(), testStream)
	if !IsJetStreamUnavailable(err) {
		t.Fatalf("err = %v, want JetStream unavailable", err)
	}
}

// Nats must offer durable publishing.
var _ broker.DurablePublisher = (*Nats)(nil)

var testMessage = &broker.Message{
	ObjectType: broker.MeshSync,
	EventType:  broker.Add,
	Object:     map[string]string{"name": "pod-a"},
}

// streamMessageCount returns how many messages the stream currently stores.
func streamMessageCount(t *testing.T, natsHandler *Nats, streamName string) uint64 {
	t.Helper()
	jetStreamClient, err := jetstream.New(natsHandler.nc)
	if err != nil {
		t.Fatalf("jetstream.New: %v", err)
	}
	stream, err := jetStreamClient.Stream(context.Background(), streamName)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	streamInfo, err := stream.Info(context.Background())
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	return streamInfo.State.Msgs
}

func TestPublishDurableDropsRepeatedKey(t *testing.T) {
	natsHandler := newTestHandler(t, true)
	ctx := context.Background()
	if err := natsHandler.EnsureStream(ctx, testStream); err != nil {
		t.Fatalf("EnsureStream: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := natsHandler.PublishDurable(ctx, "test.events", testMessage, "pod-a|v1|ADDED"); err != nil {
			t.Fatalf("PublishDurable attempt %d: %v", attempt, err)
		}
	}

	if count := streamMessageCount(t, natsHandler, testStream.StreamName); count != 1 {
		t.Fatalf("stream has %d messages, want 1", count)
	}
}

func TestPublishDurableKeepsDistinctKeys(t *testing.T) {
	natsHandler := newTestHandler(t, true)
	ctx := context.Background()
	if err := natsHandler.EnsureStream(ctx, testStream); err != nil {
		t.Fatalf("EnsureStream: %v", err)
	}

	for _, key := range []string{"pod-a|v1|ADDED", "pod-a|v2|MODIFIED"} {
		if err := natsHandler.PublishDurable(ctx, "test.events", testMessage, key); err != nil {
			t.Fatalf("PublishDurable %s: %v", key, err)
		}
	}

	if count := streamMessageCount(t, natsHandler, testStream.StreamName); count != 2 {
		t.Fatalf("stream has %d messages, want 2", count)
	}
}

func TestPublishDurableWithEmptyKeyKeepsEveryMessage(t *testing.T) {
	natsHandler := newTestHandler(t, true)
	ctx := context.Background()
	if err := natsHandler.EnsureStream(ctx, testStream); err != nil {
		t.Fatalf("EnsureStream: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := natsHandler.PublishDurable(ctx, "test.events", testMessage, ""); err != nil {
			t.Fatalf("PublishDurable attempt %d: %v", attempt, err)
		}
	}

	if count := streamMessageCount(t, natsHandler, testStream.StreamName); count != 2 {
		t.Fatalf("stream has %d messages, want 2", count)
	}
}

func TestPublishDurableWithoutStream(t *testing.T) {
	natsHandler := newTestHandler(t, true) // JetStream on, but no EnsureStream
	err := natsHandler.PublishDurable(context.Background(), "test.events", testMessage, "key")
	if !IsJetStreamUnavailable(err) {
		t.Fatalf("err = %v, want JetStream unavailable", err)
	}
}

func TestPublishDurableWithoutJetStream(t *testing.T) {
	natsHandler := newTestHandler(t, false)
	err := natsHandler.PublishDurable(context.Background(), "test.events", testMessage, "key")
	if !IsJetStreamUnavailable(err) {
		t.Fatalf("err = %v, want JetStream unavailable", err)
	}
}

// Without a stream, a core subscriber on the subject receives the publish but
// never replies, so PublishDurable times out with ErrPublishDurable instead of
// returning ErrJetStreamUnavailable. Callers must run EnsureStream first.
func TestPublishDurableWithoutStreamButCoreSubscriber(t *testing.T) {
	natsHandler := newTestHandler(t, true) // JetStream on, no EnsureStream
	received := make(chan *broker.Message, 1)
	if err := natsHandler.SubscribeWithChannel("test.events", "", received); err != nil {
		t.Fatalf("SubscribeWithChannel: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := natsHandler.PublishDurable(ctx, "test.events", testMessage, "key")
	if errors.GetCode(err) != ErrPublishDurableCode {
		t.Fatalf("err = %v, want code %s", err, ErrPublishDurableCode)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("core subscriber did not receive the message")
	}
}
