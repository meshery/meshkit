package utils

import (
	"archive/zip"
	"os"
	"path/filepath"
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

func TestExtractZip_Destination(t *testing.T) {
	destDir, _ := os.MkdirTemp("", "correct-dest-*")
	defer os.RemoveAll(destDir)

	cwd, _ := os.Getwd()

	subDirName := "target-subfolder"
	zipFile, _ := os.CreateTemp("", "test-*.zip")
	writer := zip.NewWriter(zipFile)

	_, _ = writer.Create(subDirName + "/")
	f, _ := writer.Create(filepath.Join(subDirName, "file.txt"))
	f.Write([]byte("content"))
	writer.Close()
	zipFile.Close()
	defer os.Remove(zipFile.Name())

	err := ExtractZip(destDir, zipFile.Name())
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	wrongPath := filepath.Join(cwd, subDirName)
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("BUG FOUND: Folder was created in CWD: %s", wrongPath)
		os.RemoveAll(wrongPath) // Cleanup the mess made by the bug
	}

	rightPath := filepath.Join(destDir, subDirName)
	if _, err := os.Stat(rightPath); os.IsNotExist(err) {
		t.Errorf("FAILURE: Folder was NOT created in destination: %s", rightPath)
	}
}
