package kompose

import (
	"testing"
)

// TestValidatePanicRecovery tests that Validate method recovers from panics
func TestValidatePanicRecovery(t *testing.T) {
	dc := DockerComposeFile([]byte("invalid data"))

	// This should not panic even if underlying libraries panic
	err := dc.Validate(nil)

	// We expect an error, not a panic
	if err == nil {
		t.Error("Expected an error when validating with nil schema, got nil")
	}
}

// TestConvertPanicRecovery tests that Convert function recovers from panics
func TestConvertPanicRecovery(t *testing.T) {
	// Create an invalid docker compose file that might cause panic
	dc := DockerComposeFile([]byte(""))

	// This should not panic even if underlying libraries panic
	_, err := Convert(dc)

	// We expect an error, not a panic
	if err == nil {
		t.Error("Expected an error when converting empty docker compose file, got nil")
	}
}

// TestIsManifestADockerComposePanicRecovery tests that IsManifestADockerCompose recovers from panics
func TestIsManifestADockerComposePanicRecovery(t *testing.T) {
	// Create invalid manifest that might cause panic
	manifest := []byte("")

	// This should not panic even if underlying libraries panic
	err := IsManifestADockerCompose(manifest, "")

	// We expect an error, not a panic
	if err == nil {
		t.Error("Expected an error when checking empty manifest, got nil")
	}
}
