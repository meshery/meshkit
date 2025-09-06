package registration

import (
	"fmt"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/schemas/models/v1beta1/connection"
)

// ValidateConnection validates a connection entity directly from the schemas repo
// This implements the maintainer's request to validate connection entities directly
// without using a wrapper or the Entity interface
func ValidateConnection(byt []byte) (*connection.Connection, error) {
	var conn connection.Connection
	err := encoding.Unmarshal(byt, &conn)
	if err != nil {
		return nil, fmt.Errorf("invalid connection definition: %s", err.Error())
	}

	// Basic validation
	if conn.Name == "" {
		return nil, fmt.Errorf("connection name is required")
	}
	if conn.Type == "" {
		return nil, fmt.Errorf("connection type is required")
	}
	if conn.Kind == "" {
		return nil, fmt.Errorf("connection kind is required")
	}

	return &conn, nil
}
