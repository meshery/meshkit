package utils

import (
	"archive/zip"
	"os"
	"testing"
)

func TestExtractZip(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // fileName: content
		wantErr  bool
		errMatch string
	}{
		{
			name: "Valid Extraction",
			files: map[string]string{
				"test.txt":        "hello world",
				"subdir/file.txt": "nested content",
			},
			wantErr: false,
		},
		{
			name: "Zip Slip Attack Attempt",
			files: map[string]string{
				"../outside.txt": "malicious content",
			},
			wantErr:  true,
			errMatch: "zipslip: illegal file path",
		},
		{
			name: "Skip macOS Metadata",
			files: map[string]string{
				"__MACOSX/secret": "should skip",
				"._metadata":      "should skip",
				"realfile.txt":    "should keep",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, _ := os.MkdirTemp("", "extract-test-*")
			defer os.RemoveAll(tmpDir)

			zipFile, _ := os.CreateTemp("", "test-*.zip")
			defer os.Remove(zipFile.Name())

			writer := zip.NewWriter(zipFile)
			for name, content := range tt.files {
				f, _ := writer.Create(name)
				f.Write([]byte(content))
			}
			writer.Close()
			zipFile.Close()

			err := ExtractZip(tmpDir, zipFile.Name())

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractZip() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
