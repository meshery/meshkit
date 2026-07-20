package nats

import (
	"testing"

	"github.com/meshery/meshkit/broker"
	nats "github.com/nats-io/nats.go"
)

// Nats must satisfy the broker.Handler interface, including Unsubscribe.
var _ broker.Handler = (*Nats)(nil)

func TestSubscriptionsAddTake(t *testing.T) {
	s := newSubscriptions()

	// take on an unknown subject returns nothing.
	if got := s.take("missing"); got != nil {
		t.Fatalf("take(missing) = %v, want nil", got)
	}

	// add records multiple subscriptions per subject; take returns and removes them.
	a, b := &nats.Subscription{}, &nats.Subscription{}
	s.add("subj", a)
	s.add("subj", b)
	if got := s.take("subj"); len(got) != 2 {
		t.Fatalf("take(subj) returned %d subscriptions, want 2", len(got))
	}
	// A second take is empty: the first take removed them.
	if got := s.take("subj"); got != nil {
		t.Fatalf("second take(subj) = %v, want nil", got)
	}

	// add is nil-safe for both a nil receiver and a nil subscription.
	var nilTracker *subscriptions
	nilTracker.add("x", a) // must not panic
	s.add("y", nil)
	if got := s.take("y"); len(got) != 0 {
		t.Fatalf("take(y) after adding a nil subscription = %v, want empty", got)
	}
}

func TestNatsUnsubscribeNilConnectionIsNoOp(t *testing.T) {
	// A nil handler and an uninitialized handler hold no subscriptions, so
	// Unsubscribe must be a no-op that returns nil rather than erroring.
	var nilHandler *Nats
	if err := nilHandler.Unsubscribe("subj"); err != nil {
		t.Fatalf("nil handler Unsubscribe = %v, want nil", err)
	}
	if err := (&Nats{}).Unsubscribe("subj"); err != nil {
		t.Fatalf("uninitialized handler Unsubscribe = %v, want nil", err)
	}
}
