package utils

import (
	"fmt"
	"net"
	"time"
)

func TcpCheck(ip string, port int32) bool {
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		return true
	}
	return false
}
