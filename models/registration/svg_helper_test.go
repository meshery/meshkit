package registration

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeSVGFile struct {
	writeErr error
	closeErr error
	content  string
	closed   bool
}

func (f *fakeSVGFile) WriteString(content string) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}

	f.content += content
	return len(content), nil
}

func (f *fakeSVGFile) Close() error {
	f.closed = true
	return f.closeErr
}

func TestWriteAndCloseSVG(t *testing.T) {
	t.Run("writes and closes file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.svg")
		content := "<svg>test</svg>"

		if err := writeAndCloseSVG(path, content); err != nil {
			t.Fatalf("writeAndCloseSVG() error = %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read SVG: %v", err)
		}

		if string(got) != content {
			t.Errorf("content = %q, want %q", string(got), content)
		}
	})

	t.Run("returns write error and closes file", func(t *testing.T) {
		wantErr := errors.New("write failed")
		fake := &fakeSVGFile{writeErr: wantErr}

		oldCreate := createSVGFile
		createSVGFile = func(string) (svgFile, error) {
			return fake, nil
		}
		defer func() {
			createSVGFile = oldCreate
		}()

		err := writeAndCloseSVG("test.svg", "content")

		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}

		if !fake.closed {
			t.Error("file was not closed after write failure")
		}
	})

	t.Run("returns close error", func(t *testing.T) {
		wantErr := errors.New("close failed")
		fake := &fakeSVGFile{closeErr: wantErr}

		oldCreate := createSVGFile
		createSVGFile = func(string) (svgFile, error) {
			return fake, nil
		}
		defer func() {
			createSVGFile = oldCreate
		}()

		err := writeAndCloseSVG("test.svg", "content")

		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}

		if !fake.closed {
			t.Error("file was not closed")
		}
	})
}
