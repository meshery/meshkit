package coder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meshery/meshkit/cmd/errorutil/internal/component"
	mesherr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
)

// A declaration matching ^Err[A-Z].+Code$ that has no inspectable value used to
// panic the analyzer with "index out of range [0] with length 0". It must now be
// reported as an un-analyzable entry instead.
func TestHandleFileErrorCodeDeclarationWithoutValue(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		wantEntries   int
		wantLiterals  int
		wantCallExprs int
		wantCode      string
		wantCodeIsInt bool
	}{
		{
			name:          "var without initializer is recorded, not fatal",
			src:           "package p\n\nvar ErrNoInitCode string\n",
			wantEntries:   1,
			wantLiterals:  0,
			wantCallExprs: 1,
			wantCode:      "",
			wantCodeIsInt: false,
		},
		{
			name:          "call expression value keeps its existing handling",
			src:           "package p\n\nfunc mk() string { return \"x\" }\n\nvar ErrCallCode = mk()\n",
			wantEntries:   1,
			wantLiterals:  0,
			wantCallExprs: 1,
			wantCode:      "",
			wantCodeIsInt: false,
		},
		{
			name:          "ordinary integer literal is unchanged",
			src:           "package p\n\nconst ErrGoodCode = \"meshkit-1001\"\n",
			wantEntries:   1,
			wantLiterals:  1,
			wantCallExprs: 0,
			wantCode:      "1001",
			wantCodeIsInt: true,
		},
		{
			name:          "ordinary placeholder is unchanged",
			src:           "package p\n\nconst ErrNewCode = \"replace_me\"\n",
			wantEntries:   1,
			wantLiterals:  1,
			wantCallExprs: 0,
			wantCode:      "replace_",
			wantCodeIsInt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "error.go")
			if err := os.WriteFile(path, []byte(tt.src), 0600); err != nil {
				t.Fatal(err)
			}

			infoAll := mesherr.NewInfoAll()
			comp := &component.Info{Name: "meshkit", Type: "library", NextErrorCode: 1010}

			// Must not panic, and must not return an error.
			if err := handleFile(path, false, false, infoAll, comp); err != nil {
				t.Fatalf("handleFile() error = %v, want nil", err)
			}

			if got := len(infoAll.Entries); got != tt.wantEntries {
				t.Fatalf("Entries = %d, want %d (%+v)", got, tt.wantEntries, infoAll.Entries)
			}
			if got := len(infoAll.LiteralCodes); got != tt.wantLiterals {
				t.Errorf("LiteralCodes = %d, want %d", got, tt.wantLiterals)
			}
			if got := len(infoAll.CallExprCodes); got != tt.wantCallExprs {
				t.Errorf("CallExprCodes = %d, want %d", got, tt.wantCallExprs)
			}
			if got := infoAll.Entries[0].Code; got != tt.wantCode {
				t.Errorf("Code = %q, want %q", got, tt.wantCode)
			}
			if got := infoAll.Entries[0].CodeIsInt; got != tt.wantCodeIsInt {
				t.Errorf("CodeIsInt = %v, want %v", got, tt.wantCodeIsInt)
			}
		})
	}
}

// `errorutil update` must not panic on the same input, and must not invent a
// code for a declaration it cannot rewrite.
func TestHandleFileUpdateDoesNotPanicOnValuelessDeclaration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "error.go")
	src := "package p\n\nvar ErrNoInitCode string\n\nconst ErrRealCode = \"replace_me\"\n"
	if err := os.WriteFile(path, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}

	infoAll := mesherr.NewInfoAll()
	comp := &component.Info{Name: "meshkit", Type: "library", NextErrorCode: 1010}

	if err := handleFile(path, true, false, infoAll, comp); err != nil {
		t.Fatalf("handleFile() error = %v, want nil", err)
	}
	if comp.NextErrorCode != 1011 {
		t.Errorf("NextErrorCode = %d, want 1011 (exactly one placeholder allocated)", comp.NextErrorCode)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "\"meshkit-1010\""; !strings.Contains(string(out), want) {
		t.Errorf("rewritten file does not contain %s:\n%s", want, out)
	}
	if strings.Contains(string(out), "ErrNoInitCode string = ") {
		t.Errorf("valueless declaration must not be rewritten:\n%s", out)
	}
}
