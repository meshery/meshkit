package modsync

import (
	"regexp"
	"strings"
	"testing"
)

// replaceLine matches a single replace directive inside a block, capturing the
// "from" module path and the "to" version (empty for local-path replacements).
var replaceLine = regexp.MustCompile(`^\s+(\S+)(?:\s+\S+)?\s+=>\s+\S+(?:\s+(\S+))?\s*$`)

// parseReplaces returns a map of "from" module -> list of "to" specs found in
// every replace block of a go.mod string. A module mapping to more than one
// distinct spec is exactly the "conflicting replacements" state Go rejects.
func parseReplaces(t *testing.T, gomod string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	inBlock := false
	for _, line := range strings.Split(gomod, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case !inBlock && (trim == "replace (" || trim == "replace("):
			inBlock = true
		case inBlock && trim == ")":
			inBlock = false
		case inBlock && strings.Contains(trim, "=>"):
			m := replaceLine.FindStringSubmatch(line)
			if m == nil {
				t.Fatalf("unparseable replace line: %q", line)
			}
			out[m[1]] = append(out[m[1]], m[2])
		}
	}
	return out
}

func sync(t *testing.T, src, dest string) string {
	t.Helper()
	g, err := New(strings.NewReader(src))
	if err != nil {
		t.Fatalf("New(src): %v", err)
	}
	got, err := g.SyncRequire(strings.NewReader(dest), false)
	if err != nil {
		t.Fatalf("SyncRequire: %v", err)
	}
	return got
}

// assertNoDuplicateReplaces fails if any module is replaced more than once,
// which is what makes `go mod tidy` abort with "conflicting replacements".
func assertNoDuplicateReplaces(t *testing.T, gomod string) {
	t.Helper()
	for mod, specs := range parseReplaces(t, gomod) {
		if len(specs) > 1 {
			t.Errorf("module %s replaced %d times: %v", mod, len(specs), specs)
		}
	}
}

// TestConflictingDestReplacesDeduped reproduces the release-pipeline failure:
// the destination arrives with two conflicting replaces for a module the
// source pins nowhere (github.com/vmihailenco/msgpack/v5 at v5.4.1 and v5.3.5,
// in separate blocks). The tool must collapse them to a single directive.
func TestConflictingDestReplacesDeduped(t *testing.T) {
	src := "module github.com/meshery/meshery\n\ngo 1.26.4\n"
	dest := `module github.com/meshery/extensions

go 1.26.4

replace (
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.5.4
	github.com/vmihailenco/msgpack/v5 => github.com/vmihailenco/msgpack/v5 v5.4.1
)

replace (
	github.com/vmihailenco/msgpack/v5 => github.com/vmihailenco/msgpack/v5 v5.3.5
)
`
	got := sync(t, src, dest)
	assertNoDuplicateReplaces(t, got)

	specs := parseReplaces(t, got)["github.com/vmihailenco/msgpack/v5"]
	if len(specs) != 1 {
		t.Fatalf("msgpack replaced %d times, want 1: %v", len(specs), specs)
	}
	if specs[0] != "v5.4.1" {
		t.Errorf("msgpack pinned to %s, want v5.4.1 (first occurrence wins)", specs[0])
	}
}

// TestSourceReplaceWins verifies a source-declared replace overrides a
// destination replace for the same module.
func TestSourceReplaceWins(t *testing.T) {
	src := "module github.com/meshery/meshery\n\ngo 1.26.4\n\nreplace github.com/foo/bar => github.com/foo/bar v2.0.0\n"
	dest := "module m\n\ngo 1.26.4\n\nreplace github.com/foo/bar => github.com/foo/bar v1.0.0\n"

	got := sync(t, src, dest)
	assertNoDuplicateReplaces(t, got)
	if specs := parseReplaces(t, got)["github.com/foo/bar"]; len(specs) != 1 || specs[0] != "v2.0.0" {
		t.Errorf("foo/bar = %v, want [v2.0.0]", specs)
	}
}

// TestSourceRequirePinned verifies every source require is pinned via replace
// so the plugin links against the host's exact versions.
func TestSourceRequirePinned(t *testing.T) {
	src := "module github.com/meshery/meshery\n\ngo 1.26.4\n\nrequire (\n\tgithub.com/foo/bar v3.1.0 // indirect\n)\n"
	dest := "module m\n\ngo 1.26.4\n"

	got := sync(t, src, dest)
	assertNoDuplicateReplaces(t, got)
	if specs := parseReplaces(t, got)["github.com/foo/bar"]; len(specs) != 1 || specs[0] != "v3.1.0" {
		t.Errorf("foo/bar = %v, want [v3.1.0]", specs)
	}
}

// TestDestOnlyReplacePreserved verifies an extension-specific replace the
// source does not touch — including a local-path replace — survives.
func TestDestOnlyReplacePreserved(t *testing.T) {
	src := "module github.com/meshery/meshery\n\ngo 1.26.4\n"
	dest := `module m

go 1.26.4

replace github.com/meshery/meshery v0.5.0-rc => ../../meshery

replace github.com/only/dest => github.com/only/dest v1.2.3
`
	got := sync(t, src, dest)
	assertNoDuplicateReplaces(t, got)

	specs := parseReplaces(t, got)
	if v := specs["github.com/only/dest"]; len(v) != 1 || v[0] != "v1.2.3" {
		t.Errorf("only/dest = %v, want [v1.2.3]", v)
	}
	if !strings.Contains(got, "github.com/meshery/meshery v0.5.0-rc => ../../meshery") {
		t.Errorf("local-path replace to ../../meshery was not preserved:\n%s", got)
	}
}

// TestIdempotent verifies a second sync of already-synced output is a no-op,
// proving the tool no longer grows the file with duplicate blocks on repeat
// runs (the mechanism behind the original corruption).
func TestIdempotent(t *testing.T) {
	src := `module github.com/meshery/meshery

go 1.26.4

require (
	github.com/foo/bar v3.1.0 // indirect
)

replace github.com/baz/qux => github.com/baz/qux v1.0.0
`
	dest := `module m

go 1.26.4

replace github.com/only/dest => github.com/only/dest v1.2.3

replace (
	github.com/vmihailenco/msgpack/v5 => github.com/vmihailenco/msgpack/v5 v5.4.1
)

replace (
	github.com/vmihailenco/msgpack/v5 => github.com/vmihailenco/msgpack/v5 v5.3.5
)
`
	first := sync(t, src, dest)
	assertNoDuplicateReplaces(t, first)
	second := sync(t, src, first)
	if first != second {
		t.Errorf("SyncRequire is not idempotent.\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}
