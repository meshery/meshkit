package converter_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/meshery/meshkit/converter"
	meshkiterr "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
)

func TestHelmConverter_Convert(t *testing.T) {
	testCases := []struct {
		name               string
		filePath           string
		expectError        bool
		expectedErrCode    string
		checkManifest      bool
		wantManifestSubstr string
	}{
		{
			name:               "Valid Edge Firewall Relationship Deployment",
			filePath:           "./samples/edge-firewall-relationship.yml",
			expectError:        false,
			checkManifest:      true,
			wantManifestSubstr: "apiVersion",
		},
		{
			name:            "Missing Pattern File",
			filePath:        "./samples/does-not-exist.yaml",
			expectError:     true,
			expectedErrCode: "meshkit-11315",
		},
		{
			name:            "Invalid YAML",
			filePath:        "./samples/invalid.yaml",
			expectError:     true,
			expectedErrCode: "meshkit-11315",
		},
		{
			name:            "Missing Name",
			filePath:        "./samples/missing-name.yaml",
			expectError:     true,
			expectedErrCode: "meshkit-11317",
		},
		{
			name:            "Missing Version",
			filePath:        "./samples/missing-version.yaml",
			expectError:     true,
			expectedErrCode: "meshkit-11317",
		},
		{
			name:            "Empty File",
			filePath:        "./samples/empty.yaml",
			expectError:     true,
			expectedErrCode: "meshkit-11315",
		},
		{
			name:               "No Components",
			filePath:           "./samples/no-components.yaml",
			expectError:        false,
			checkManifest:      true,
			wantManifestSubstr: "",
		},
		{
			name:            "Invalid K8s Component",
			filePath:        "./samples/invalid-k8s.yaml",
			expectError:     true,
			expectedErrCode: "meshkit-11315",
		},
	}

	hc := &converter.HelmConverter{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data []byte
			var err error
			if _, err = os.Stat(tc.filePath); err == nil {
				data, err = os.ReadFile(tc.filePath)
				if err != nil {
					t.Fatalf("Failed to read test file: %v", err)
				}
			} else {
				data = nil
			}

			chartData, err := hc.Convert(string(data))

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				if tc.expectedErrCode != "" {
					code := meshkiterr.GetCode(err)
					if code != tc.expectedErrCode {
						t.Errorf("Expected error code %q, got %q", tc.expectedErrCode, code)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(chartData) == 0 {
				t.Errorf("Expected non-empty chart data")
			}
			if tc.checkManifest {
				found, content := extractManifestFromChart([]byte(chartData))
				if !found {
					t.Errorf("manifest.yaml not found in chart")
				}
				if tc.wantManifestSubstr != "" && !strings.Contains(content, tc.wantManifestSubstr) {
					t.Errorf("manifest.yaml does not contain expected content: %q", tc.wantManifestSubstr)
				}
			}
		})
	}
}

func extractManifestFromChart(chartData []byte) (bool, string) {
	gr, err := gzip.NewReader(bytes.NewReader(chartData))
	if err != nil {
		return false, ""
	}
	defer utils.SafeClose(gr)
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, ""
		}
		if strings.HasSuffix(hdr.Name, "templates/manifest.yaml") {
			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, tr); err != nil {
				return false, ""
			}
			return true, buf.String()
		}
	}
	return false, ""
}
