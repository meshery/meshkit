// Package iacext holds the file-extension table that describes which
// extensions a kustomization may legitimately arrive as.
//
// The table lives in its own leaf package (rather than in "files") so that both
// "files" and "utils/walker" can read it: "files" already imports
// "utils/walker", so a table owned by "files" and consumed by the walker would
// close an import cycle.
package iacext

// ValidKustomizeFileExtensions enumerates the extensions a kustomization may
// arrive as, either as a single kustomization file or as an archive of one.
var ValidKustomizeFileExtensions = map[string]bool{
	".yml":    true, // single kustomization.yml file
	".yaml":   true,
	".tar":    true,
	".tgz":    true,
	".gz":     true,
	".tar.gz": true,
	".zip":    true,
}
