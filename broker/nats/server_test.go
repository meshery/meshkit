package nats

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// runServer starts an in-process NATS server on a random port and shuts it
// down when the test ends. enableJetStream turns JetStream on or off.
func runServer(t *testing.T, enableJetStream bool) *server.Server {
	t.Helper()
	serverOptions := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: enableJetStream,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	}
	natsServer, err := server.NewServer(serverOptions)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	natsServer.Start()
	t.Cleanup(natsServer.Shutdown)
	if !natsServer.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	return natsServer
}

func TestNewConnectsToServer(t *testing.T) {
	natsServer := runServer(t, false)

	handler, err := New(Options{URLS: []string{natsServer.ClientURL()}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(handler.CloseConnection)

	if !handler.IsConnected() {
		t.Fatal("IsConnected() = false, want true")
	}
}
