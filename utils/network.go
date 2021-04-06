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

func TcpCheck(hp *HostPort) bool {
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", hp.Address, hp.Port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		return true
	}
	return false
}
