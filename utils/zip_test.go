package utils

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
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
			tmpDir, err := os.MkdirTemp("", "extract-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)
			zipPath := filepath.Join(t.TempDir(), "test.zip")
			f, err := os.Create(zipPath)
			if err != nil {
				t.Fatal(err)
			}

			writer := zip.NewWriter(f)
			for name, content := range tt.files {
				w, err := writer.Create(name)
				if err != nil {
					continue
				}
				w.Write([]byte(content))
			}
			writer.Close()
			f.Close()

			err = ExtractZip(tmpDir, zipPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractZip() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMatch != "" {
				if !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("error %q does not match %q", err.Error(), tt.errMatch)
				}
				return
			}

			for name, expectedContent := range tt.files {
				if strings.HasPrefix(filepath.Base(name), "._") || filepath.Base(name) == "__MACOSX" {
					continue
				}

				path := filepath.Join(tmpDir, name)
				gotContent, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("Expected file %s missing: %v", name, err)
					continue
				}
				if string(gotContent) != expectedContent {
					t.Errorf("File %s: got %q, want %q", name, string(gotContent), expectedContent)
				}
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

	header := &zip.FileHeader{
		Name:   subDirName + "/",
		Method: zip.Store,
	}
	header.SetMode(0755)
	_, _ = writer.CreateHeader(header)

	f, _ := writer.Create(filepath.Join(subDirName, "file.txt"))
	f.Write([]byte("content"))

	writer.Close()
	zipFile.Close()
	defer os.Remove(zipFile.Name())

	extractionErr := ExtractZip(destDir, zipFile.Name())
	wrongPath := filepath.Join(cwd, subDirName)
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("BUG FOUND: Folder was created in CWD: %s", wrongPath)
		os.RemoveAll(wrongPath)
	}

	rightPath := filepath.Join(destDir, subDirName)
	if _, err := os.Stat(rightPath); os.IsNotExist(err) {
		t.Errorf("FAILURE: Folder was NOT created in destination: %s", rightPath)
	}
	if extractionErr != nil {
		t.Fatalf("Extraction failed: %v", extractionErr)
	}

}
