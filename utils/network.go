package utils

import (
	"fmt"
	"net"
	"time"
)

// Endpoint represents the structure for an endpoint
type Endpoint struct {
	Name     string
	Internal *HostPort
	External *HostPort
}

type HostPort struct {
	Address string
	Port    int32
}

func (hp *HostPort) String() string {
	return fmt.Sprintf("%s:%d", hp.Address, hp.Port)
}

type MockOptions struct {
	DesiredEndpoint string
}

func TcpCheck(hp *HostPort, mock *MockOptions) bool {
	timeout := 5 * time.Second

	// For mocking output
	if mock != nil {
		return mock.DesiredEndpoint == fmt.Sprintf("%s:%d", hp.Address, hp.Port)
	}

	conn, _ := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", hp.Address, hp.Port), timeout)
	return conn != nil
}
