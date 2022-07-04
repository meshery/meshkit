package manifests

import "testing"

type testcase struct {
	input          string
	expectedOutput string
}

func TestFormatToReadableString(t *testing.T) {
	testCases := []testcase{
		{
			input:          "APIService",
			expectedOutput: "API Service",
		},
		{
			input:          "TrafficSplit",
			expectedOutput: "Traffic Split",
		},
		{
			input:          "CIDRsRanges",
			expectedOutput: "CIDRs Ranges",
		},
		{
			input:          "IPFamiliesWithIPs",
			expectedOutput: "IP Families With IPs",
		},
		{
			input:          "idConnectedToIPs",
			expectedOutput: "id Connected To IPs",
		},
		{
			input:          "Mesh Sync",
			expectedOutput: "MeshSync",
		},
	}
	for _, tt := range testCases {
		output := FormatToReadableString(tt.input)
		if tt.expectedOutput != output {
			t.Fatalf("Expected %s, got %s", tt.expectedOutput, output)
		}
	}
}
