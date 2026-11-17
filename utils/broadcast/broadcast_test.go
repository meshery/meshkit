package broadcast

import (
	"sync"
	"testing"
	"time"

	"github.com/gofrs/uuid"
)

func newTestMessage(t *testing.T, msgType string, data interface{}) BroadcastMessage {
	t.Helper()
	id, err := uuid.NewV4()
	if err != nil {
		t.Fatalf("failed to generate uuid: %v", err)
	}
	return BroadcastMessage{
		Id:     id,
		Source: OperatorSyncChannel,
		Type:   msgType,
		Data:   data,
		Time:   time.Now().UTC(),
	}
}

func TestBroadcaster_SingleAndMultipleSubscribers(t *testing.T) {
	b := NewBroadcaster(4)
	defer func() {
		if err := b.Close(); err != nil {
			t.Errorf("unexpected error closing broadcaster: %v", err)
		}
	}()

	sub1 := make(chan BroadcastMessage, 1)
	sub2 := make(chan BroadcastMessage, 1)

	b.Register(sub1)
	b.Register(sub2)

	want := newTestMessage(t, "sync", "payload-1")
	b.Submit(want)

	for i, sub := range []chan BroadcastMessage{sub1, sub2} {
		select {
		case got := <-sub:
			if got.Id != want.Id || got.Source != want.Source || got.Type != want.Type || got.Data != want.Data {
				t.Errorf("subscriber %d got %+v, want %+v", i+1, got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for message on subscriber %d", i+1)
		}
	}
}

func TestBroadcaster_UnregisterStopsDelivery(t *testing.T) {
	b := NewBroadcaster(4)
	defer func() { _ = b.Close() }()

	sub1 := make(chan BroadcastMessage, 2)
	sub2 := make(chan BroadcastMessage, 2)

	b.Register(sub1)
	b.Register(sub2)

	first := newTestMessage(t, "first", 1)
	b.Submit(first)

	// Drain first message on both subscribers before unregistering sub2.
	<-sub1
	<-sub2

	b.Unregister(sub2)

	second := newTestMessage(t, "second", 2)
	b.Submit(second)

	select {
	case got := <-sub1:
		if got.Id != second.Id {
			t.Errorf("sub1 got %v, want %v", got.Id, second.Id)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second message on sub1")
	}

	select {
	case unexpected := <-sub2:
		t.Errorf("unregistered sub2 unexpectedly received message: %+v", unexpected)
	case <-time.After(50 * time.Millisecond):
		// Expected: no message delivered to unregistered channel.
	}
}

func TestBroadcaster_NilSubmitDoesNotPanic(t *testing.T) {
	var b *broadcaster
	msg := newTestMessage(t, "nil-check", nil)

	// Should safely no-op without panicking.
	b.Submit(msg)
}

func TestBroadcaster_ConcurrentSubmitAndRegister(t *testing.T) {
	const (
		numSubscribers = 5
		numMessages    = 20
	)

	b := NewBroadcaster(numMessages)
	defer func() { _ = b.Close() }()

	subs := make([]chan BroadcastMessage, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		subs[i] = make(chan BroadcastMessage, numMessages)
		b.Register(subs[i])
	}

	var wg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b.Submit(newTestMessage(t, "concurrent", idx))
		}(i)
	}
	wg.Wait()

	for subIdx, sub := range subs {
		for msgIdx := 0; msgIdx < numMessages; msgIdx++ {
			select {
			case <-sub:
			case <-time.After(2 * time.Second):
				t.Fatalf("subscriber %d received only %d/%d messages", subIdx, msgIdx, numMessages)
			}
		}
	}
}
