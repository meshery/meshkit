package nats

import (
	"errors"
	"sync"
	"testing"

	"github.com/meshery/meshkit/broker"
	natsio "github.com/nats-io/nats.go"
)

type mockNatsConn struct {
	servers         []string
	name            string
	publishErr      error
	queueSubErr     error
	closed          bool
	drainErr        error
	publishMessages [][]byte
}

func (m *mockNatsConn) Servers() []string { return m.servers }
func (m *mockNatsConn) Publish(subject string, data []byte) error {
	m.publishMessages = append(m.publishMessages, data)
	return m.publishErr
}
func (m *mockNatsConn) QueueSubscribe(subject, queue string, cb func(msg *natsio.Msg)) (*natsio.Subscription, error) {
	if m.queueSubErr != nil {
		return nil, m.queueSubErr
	}
	// Simulate a message
	msg := &natsio.Msg{Data: []byte(`{"ObjectType":"request-payload","EventType":"ADDED"}`)}
	cb(msg)
	return nil, nil
}
func (m *mockNatsConn) Drain() error         { return m.drainErr }
func (m *mockNatsConn) Close()               { m.closed = true }
func (m *mockNatsConn) Opts() natsio.Options { return natsio.Options{Name: m.name} }

// Ensure mockNatsConn implements NatsConn
var _ NatsConn = (*mockNatsConn)(nil)

func newTestNats(mock NatsConn) *Nats {
	return &Nats{conn: mock, wg: &sync.WaitGroup{}}
}

func TestConnectedEndpoints(t *testing.T) {
	mock := &mockNatsConn{servers: []string{"nats://localhost:4222", "nats://other:4222"}}
	n := newTestNats(mock)
	endpoints := n.ConnectedEndpoints()
	if len(endpoints) != 2 || endpoints[0] != "localhost:4222" {
		t.Errorf("unexpected endpoints: %v", endpoints)
	}
}

func TestInfo(t *testing.T) {
	n := &Nats{conn: nil}
	if n.Info() != broker.NotConnected {
		t.Error("expected NotConnected when conn is nil")
	}
	mock := &mockNatsConn{name: "test-conn"}
	n = newTestNats(mock)
	if n.Info() != "test-conn" {
		t.Errorf("expected 'test-conn', got %s", n.Info())
	}
}

func TestCloseConnection(t *testing.T) {
	mock := &mockNatsConn{}
	n := newTestNats(mock)
	n.CloseConnection()
	if !mock.closed {
		t.Error("expected connection to be closed")
	}
}

func TestPublish(t *testing.T) {
	mock := &mockNatsConn{}
	n := newTestNats(mock)
	msg := &broker.Message{ObjectType: broker.Request, EventType: broker.Add}
	err := n.Publish("subject", msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Test error on marshal
	err = n.Publish("subject", nil)
	if err == nil {
		t.Error("expected error for nil message")
	}
	// Test error on publish
	mock.publishErr = errors.New("publish error")
	msg = &broker.Message{ObjectType: broker.Request, EventType: broker.Add}
	err = n.Publish("subject", msg)
	if err == nil {
		t.Error("expected error from publish")
	}
}

func TestPublishWithChannel(t *testing.T) {
	mock := &mockNatsConn{}
	n := newTestNats(mock)
	ch := make(chan *broker.Message, 1)
	ch <- &broker.Message{ObjectType: broker.Request, EventType: broker.Add}
	close(ch)
	err := n.PublishWithChannel("subject", ch)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubscribe(t *testing.T) {
	mock := &mockNatsConn{}
	n := newTestNats(mock)
	n.wg = &sync.WaitGroup{} // ensure wg is set
	var msg []byte
	err := n.Subscribe("subject", "queue", msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Test error on subscribe
	mock.queueSubErr = errors.New("subscribe error")
	err = n.Subscribe("subject", "queue", msg)
	if err == nil {
		t.Error("expected error from subscribe")
	}
}

func TestSubscribeWithChannel(t *testing.T) {
	mock := &mockNatsConn{}
	n := newTestNats(mock)
	ch := make(chan *broker.Message, 1)
	err := n.SubscribeWithChannel("subject", "queue", ch)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Test error on subscribe
	mock.queueSubErr = errors.New("subscribe error")
	err = n.SubscribeWithChannel("subject", "queue", ch)
	if err == nil {
		t.Error("expected error from subscribe")
	}
}

func TestDeepCopy(t *testing.T) {
	n := &Nats{conn: nil, wg: &sync.WaitGroup{}}
	copy := n.DeepCopy()
	if copy == nil || copy == n {
		t.Error("DeepCopy did not create a new instance")
	}
}

func TestDeepCopyObject(t *testing.T) {
	n := &Nats{conn: nil, wg: &sync.WaitGroup{}}
	obj := n.DeepCopyObject()
	if obj == nil {
		t.Error("DeepCopyObject returned nil")
	}
}

func TestIsEmpty(t *testing.T) {
	var n *Nats
	if !n.IsEmpty() {
		t.Error("expected true for nil receiver")
	}
	n = &Nats{}
	if !n.IsEmpty() {
		t.Error("expected true for empty Nats struct")
	}
	mock := &mockNatsConn{}
	n = &Nats{conn: mock}
	if n.IsEmpty() {
		t.Error("expected false for non-empty Nats struct")
	}
}
