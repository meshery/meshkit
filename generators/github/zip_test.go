package github

import (
	"bufio"
	"io"
	"path/filepath"
	"testing"

	"github.com/meshery/meshkit/utils/walker"
)

func TestFileInterceptorErrorsOnBadZip(t *testing.T) {
	br := bufio.NewWriter(io.Discard)
	fi := fileInterceptor(br)

	tmp := t.TempDir()
	bad := filepath.Join(tmp, "~/Downloads/Archive.zip")
	// _ = os.WriteFile(bad, []byte("not a zip"), 0644)

	f := walker.File{Path: bad, Name: "Archive.zip"}
	if err := fi(f); err == nil {
		t.Fatalf("expected error processing bad zip, got nil")
	}
}
