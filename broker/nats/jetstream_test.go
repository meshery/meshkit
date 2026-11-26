package nats

import (
	"context"
	"testing"
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

func TestEnsureStreamWithoutJetStream(t *testing.T) {
	natsHandler := newTestHandler(t, false)
	err := natsHandler.EnsureStream(context.Background(), testStream)
	if !IsJetStreamUnavailable(err) {
		t.Fatalf("err = %v, want JetStream unavailable", err)
	}
}
