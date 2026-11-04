package broadcast

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBroadcasterSlowSubscriberDoesNotBlockOthers(t *testing.T) {
	b := NewBroadcaster(10)

	// A subscriber that never drains its channel.
	slow := make(chan BroadcastMessage)
	b.Register(slow)

	// A subscriber that does drain, buffered so Submit below doesn't need a
	// live reader yet.
	fast := make(chan BroadcastMessage, 1)
	b.Register(fast)

	b.Submit(BroadcastMessage{Type: "first"})

	// The fast subscriber must still receive the message even though the
	// slow one never reads it.
	select {
	case got := <-fast:
		assert.Equal(t, "first", got.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("fast subscriber never received the broadcast message")
	}

	// Register/Unregister go through the same goroutine as broadcast(), so
	// they must not be blocked by the stalled slow subscriber either.
	for _, op := range []struct {
		name string
		fn   func()
	}{
		{"Register", func() { b.Register(make(chan BroadcastMessage, 1)) }},
		{"Unregister", func() { b.Unregister(slow) }},
	} {
		done := make(chan struct{})
		go func() { op.fn(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s blocked for >2s behind the stalled subscriber", op.name)
		}
	}
}
