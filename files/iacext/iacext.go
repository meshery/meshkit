// Package iacext holds the file-extension tables that describe which archive
// and plain-text extensions each IaC file type may legitimately arrive as.
//
// The tables live in their own leaf package (rather than in "files") so that
// both "files" and "utils/walker" can read them: "files" already imports
// "utils/walker", so a table owned by "files" and consumed by the walker would
// close an import cycle.
package iacext

// ValidHelmChartFileExtensions enumerates the archive extensions a Helm chart
// may be packaged as.
var ValidHelmChartFileExtensions = map[string]bool{
	".tar":    true,
	".tgz":    true,
	".gz":     true,
	".tar.gz": true,
	".zip":    true,
}

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
