package events

import (
	"sync"
	"testing"
	"time"
)

// awaitDelivery returns the next value received on ch and true if it arrived
// within d; on timeout it returns nil and false.
func awaitDelivery(t *testing.T, ch chan interface{}, d time.Duration) (interface{}, bool) {
	t.Helper()
	select {
	case v := <-ch:
		return v, true
	case <-time.After(d):
		return nil, false
	}
}

func TestEventStreamer_SubscribePublishReceive(t *testing.T) {
	s := NewEventStreamer()
	ch := make(chan interface{}, 1)
	s.Subscribe(ch)

	s.Publish("hello")

	got, ok := awaitDelivery(t, ch, 200*time.Millisecond)
	if !ok {
		t.Fatal("expected published value, got timeout")
	}
	if got != "hello" {
		t.Fatalf("expected \"hello\", got %v", got)
	}
}

func TestEventStreamer_Unsubscribe(t *testing.T) {
	tests := []struct {
		name            string
		unsubscribeOps  int  // how many times to call Unsubscribe on the channel
		wantDelivery    bool // whether Publish should reach the channel afterward
	}{
		{
			name:           "unsubscribe stops delivery",
			unsubscribeOps: 1,
			wantDelivery:   false,
		},
		{
			name:           "double-unsubscribe is a safe no-op",
			unsubscribeOps: 2,
			wantDelivery:   false,
		},
		{
			name:           "no unsubscribe leaves delivery intact",
			unsubscribeOps: 0,
			wantDelivery:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewEventStreamer()
			ch := make(chan interface{}, 1)
			s.Subscribe(ch)
			for i := 0; i < tc.unsubscribeOps; i++ {
				s.Unsubscribe(ch)
			}

			s.Publish("payload")

			_, delivered := awaitDelivery(t, ch, 100*time.Millisecond)
			if delivered != tc.wantDelivery {
				t.Fatalf("delivered=%v, want %v", delivered, tc.wantDelivery)
			}
		})
	}
}

func TestEventStreamer_UnsubscribeUnknownChannel(t *testing.T) {
	s := NewEventStreamer()
	subscribed := make(chan interface{}, 1)
	stranger := make(chan interface{}, 1)
	s.Subscribe(subscribed)

	// Unsubscribing a channel that was never subscribed must be a no-op
	// and must not drop the real subscriber.
	s.Unsubscribe(stranger)

	s.Publish("payload")

	got, ok := awaitDelivery(t, subscribed, 200*time.Millisecond)
	if !ok {
		t.Fatal("real subscriber did not receive payload after no-op Unsubscribe")
	}
	if got != "payload" {
		t.Fatalf("expected \"payload\", got %v", got)
	}
}

func TestEventStreamer_UnsubscribeOneOfMany(t *testing.T) {
	s := NewEventStreamer()
	kept := make(chan interface{}, 1)
	dropped := make(chan interface{}, 1)
	s.Subscribe(kept)
	s.Subscribe(dropped)
	s.Unsubscribe(dropped)

	s.Publish("payload")

	got, ok := awaitDelivery(t, kept, 200*time.Millisecond)
	if !ok {
		t.Fatal("kept subscriber did not receive payload")
	}
	if got != "payload" {
		t.Fatalf("expected \"payload\", got %v", got)
	}
	if _, delivered := awaitDelivery(t, dropped, 100*time.Millisecond); delivered {
		t.Fatal("dropped subscriber received payload after Unsubscribe")
	}
}

func TestEventStreamer_UnsubscribeConcurrent(t *testing.T) {
	// Exercises the mutex: many goroutines Subscribe, Unsubscribe, and
	// Publish against the SAME channel concurrently — in interleaved
	// orderings — must not corrupt the subscriber slice or race on
	// Publish's per-channel sender goroutines.
	s := NewEventStreamer()
	// Buffer is sized for the worst case: rounds*rounds sends (each of
	// the `rounds` Publish calls fans out to up to `rounds` subscribers).
	// Over-sizing is cheap; under-sizing would block a sender goroutine
	// past the test, which -race flags as a leak.
	const rounds = 32
	shared := make(chan interface{}, rounds*rounds)

	var wg sync.WaitGroup
	wg.Add(rounds * 3)
	for i := 0; i < rounds; i++ {
		go func() { defer wg.Done(); s.Subscribe(shared) }()
		go func() { defer wg.Done(); s.Unsubscribe(shared) }()
		go func() { defer wg.Done(); s.Publish("churn") }()
	}
	wg.Wait()

	// Post-churn the broadcaster must still be usable. Detach any
	// residual `shared` subscriptions left by the race so they don't
	// intercept the final publish, then verify a fresh Subscribe
	// receives a fresh Publish.
	s.Unsubscribe(shared)
	final := make(chan interface{}, 1)
	s.Subscribe(final)
	s.Publish("final")

	if got, ok := awaitDelivery(t, final, 200*time.Millisecond); !ok || got != "final" {
		t.Fatalf("final subscriber did not receive: got=%v ok=%v", got, ok)
	}
}
