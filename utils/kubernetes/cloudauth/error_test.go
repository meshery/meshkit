package cloudauth

import (
	stderrors "errors"
	"strings"
	"testing"
)

// MeshKit's Error() joins LongDescription entries with ".", so an entry that
// already ends with a period renders as a doubled ".." in the user-visible
// message. Every cloudauth error must render cleanly.
func TestErrorMessagesHaveNoDoubledPeriods(t *testing.T) {
	cause := stderrors.New("underlying cause")
	cases := map[string]error{
		"ErrInProcessCloudAuthUnavailable": ErrInProcessCloudAuthUnavailable(cause, ProviderEKS, "aws"),
		"ErrInProcessCloudAuthFallback":    ErrInProcessCloudAuthFallback(cause, ProviderGKE, "gke-gcloud-auth-plugin"),
		"ErrAWSCredentialsUnavailable":     ErrAWSCredentialsUnavailable(cause, "prod"),
		"ErrEKSRegionUnresolved":           ErrEKSRegionUnresolved("prod"),
		"ErrMintEKSToken":                  ErrMintEKSToken(cause, "prod"),
		"ErrGCPCredentialsUnavailable":     ErrGCPCredentialsUnavailable(cause),
		"ErrGCPServiceAccountKey":          ErrGCPServiceAccountKey(cause),
		"ErrUnsupportedCloudProvider":      ErrUnsupportedCloudProvider("azure"),
		// Nested: the unavailable error wrapping the region error, as Apply
		// produces when an EKS stanza has no resolvable region.
		"ErrInProcessCloudAuthUnavailable(ErrEKSRegionUnresolved)": ErrInProcessCloudAuthUnavailable(ErrEKSRegionUnresolved("prod"), ProviderEKS, "aws"),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if message := err.Error(); strings.Contains(message, "..") {
				t.Fatalf("error message contains a doubled period: %q", message)
			}
		})
	}
}
