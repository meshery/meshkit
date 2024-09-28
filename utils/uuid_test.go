package utils

import (
	"testing"
)

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

func isHex(char byte) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')
}
