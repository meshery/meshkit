package kompose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCheck(t *testing.T) {
	tests := []struct {
		name    string
		input   DockerComposeFile
		wantErr bool
	}{
		{name: "supported version", input: DockerComposeFile("version: \"3.9\"\nservices: {}"), wantErr: false},
		{name: "unsupported version", input: DockerComposeFile("version: \"4.0\"\nservices: {}"), wantErr: true},
		{name: "missing version", input: DockerComposeFile("services: {}"), wantErr: false},
		{name: "invalid version", input: DockerComposeFile("version: invalid\nservices: {}"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := versionCheck(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("versionCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatComposeFile(t *testing.T) {
	manifest := DockerComposeFile("version: 3.3\nservices: {}\n")
	formatComposeFile(&manifest)

	if string(manifest) != "version: \"3.3\"\n" {
		t.Errorf("formatComposeFile() = %q, want quoted version", manifest)
	}
}

func TestConvertValidComposeFile(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".meshery"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	compose := DockerComposeFile("version: \"3.8\"\nservices:\n  web:\n    image: nginx\n    ports:\n      - \"8080:80\"\n")
	result, err := Convert(compose)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if !strings.Contains(result, "kind: Service") || !strings.Contains(result, "kind: Deployment") {
		t.Errorf("Convert() output missing expected resources: %s", result)
	}
}
