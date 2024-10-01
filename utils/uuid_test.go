package utils

import (
	"testing"
)

// TestNewUUID tests the NewUUID function to ensure that it returns
// a valid UUID string of the expected length and format
func TestNewUUID(t *testing.T) {
	uuidStr, err := NewUUID()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(uuidStr) != 36 {
		t.Fatalf("expected UUID length of 36, got %d", len(uuidStr))
	}

	expectedFormat := "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	if !matchesFormat(uuidStr, expectedFormat) {
		t.Fatalf("UUID does not match expected format, got %v", uuidStr)
	}
}

// matchesFormat checks whether the given UUID string
// matches the expected format
func matchesFormat(uuidStr, format string) bool {
	if len(uuidStr) != len(format) {
		return false
	}
	for i := range uuidStr {
		if format[i] == 'x' && !isHex(uuidStr[i]) {
			return false
		} else if format[i] == '-' && uuidStr[i] != '-' {
			return false
		}
	}
	return true
}

// isHex checks whether the given character is hexadecimal or not
func isHex(char byte) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')
}
