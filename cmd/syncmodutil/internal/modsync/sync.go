package modsync

import (
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
)

const RequirePattern = `require\s?\((\n|.)*?\)`
const ReplacePattern = `replace\s?\((\n|.)*?\)`

var RequirePatternRegex *regexp.Regexp
var ReplacePatternRegex *regexp.Regexp

func init() {
	var err error
	RequirePatternRegex, err = regexp.Compile(RequirePattern)
	if err != nil {
		log.Fatal("failed to compile require pattern regex")
	}
	ReplacePatternRegex, err = regexp.Compile(ReplacePattern)
	if err != nil {
		log.Fatal("failed to compile replace pattern regex")
	}
}

type Package struct {
	Name     string
	Version  string
	Indirect bool
}

type GoMod struct {
	ReplacedVersions [][]Package //Each subarray will have two packages. A "from package" and "to package"
	RequiredVersions []Package
}

// For debugging
func (g *GoMod) PrintRequiredVersions() {
	for _, v := range g.RequiredVersions {
		fmt.Printf("package=%s ;version=%s\n", v.Name, v.Version)
	}
}

// For debugging
func (g *GoMod) PrintReplacedVersions() {
	for _, v := range g.ReplacedVersions {
		fmt.Printf("%s replaced by %s\n", v[0].Name+v[0].Version, v[1].Name+v[1].Version)
	}
}
func (g *GoMod) SyncRequire(f io.Reader, throwerr bool) (gomod string, err error) {
	b, err := io.ReadAll(f)
	if err != nil {
		return string(b), err
	}
	data := strings.Split(string(b), "\n")
	for _, required := range g.RequiredVersions {
		for i, d := range data {
			if !strings.Contains(d, "=>") && strings.Contains(d, required.Name+" ") && !strings.Contains(d, required.Version) {
				if throwerr {
					return "", fmt.Errorf("version mismatch for %s. Meshery has: %s but extension has %s", required.Name, required.Version, d)
				}
				indirect := strings.Contains(d, "//indirect")
				updateVersion := "\t" + required.Name + " " + required.Version
				if indirect {
					updateVersion += " //indirect"
				}
				data[i] = updateVersion
			}
		}
	}

	// Emit a replace block that pins every package in the source (host)
	// module graph to its exact source-selected version. Pinning via
	// `replace` — rather than relying on `require` alone — is required for
	// Go plugin ABI compatibility. Without it, the `go mod tidy` step that
	// callers run after this tool can upgrade transitive dependencies past
	// what the host binary is linked against (for example, a direct
	// dependency on github.com/99designs/gqlgen pulling in a newer
	// github.com/vektah/gqlparser/v2 than the host uses), causing
	// plugin.Open to fail at runtime with:
	//   "plugin was built with a different version of package <pkg>".
	// Because replace directives override require resolution, this
	// guarantees the destination module is compiled against the exact same
	// versions as the source. Source-declared replaces take precedence over
	// pinning and are emitted verbatim.
	replacedMods := make(map[string]bool, len(g.ReplacedVersions))
	for _, r := range g.ReplacedVersions {
		if len(r) >= 1 {
			replacedMods[r[0].Name] = true
		}
	}

	// Compute the set of modules we will emit a replace directive for, so
	// that any pre-existing replace in the destination for the same module
	// can be stripped first. Leaving both in place would cause `go mod tidy`
	// to fail with "multiple replacements for <module>".
	emitModules := make(map[string]bool, len(g.RequiredVersions)+len(g.ReplacedVersions))
	for _, required := range g.RequiredVersions {
		if !replacedMods[required.Name] {
			emitModules[required.Name] = true
		}
	}
	for _, r := range g.ReplacedVersions {
		if len(r) >= 1 {
			emitModules[r[0].Name] = true
		}
	}
	data = stripConflictingReplaces(data, emitModules)

	if len(g.RequiredVersions) > 0 || len(g.ReplacedVersions) > 0 {
		data = append(data, "replace (")
	}

	pinned := make(map[string]bool, len(g.RequiredVersions))
	for _, required := range g.RequiredVersions {
		if pinned[required.Name] || replacedMods[required.Name] {
			continue
		}
		pinned[required.Name] = true
		data = append(data, fmt.Sprintf("\t%s => %s %s", required.Name, required.Name, required.Version))
	}

	// Add all the replaced versions from source to destination. Running go
	// mod tidy after the utility will perform the cleanup in the destination
	// go.mod and remove unused entries. Instead of trying to intelligently
	// perform diffs, it is better to let `go mod tidy` do the cleanup.
	for _, replaced := range g.ReplacedVersions {
		data = append(data, formatReplaceLine(replaced))
	}

	if len(g.RequiredVersions) > 0 || len(g.ReplacedVersions) > 0 {
		data = append(data, ")")
	}
	gomod = strings.Join(data, "\n")
	return
}

// stripConflictingReplaces removes any replace directive from the
// destination go.mod lines whose "from" module is in the conflicts set.
// Both forms are handled: single-line (`replace foo => bar v1`) and lines
// inside an existing `replace (...)` block. Replaces for modules not in
// conflicts are preserved — including destination-specific overrides that
// the source does not touch. Empty replace blocks that may be left behind
// are valid go.mod syntax; `go mod tidy` cleans them up.
func stripConflictingReplaces(data []string, conflicts map[string]bool) []string {
	if len(conflicts) == 0 {
		return data
	}
	out := make([]string, 0, len(data))
	inReplaceBlock := false
	for _, line := range data {
		trim := strings.TrimSpace(line)

		if !inReplaceBlock && (trim == "replace (" || trim == "replace(") {
			inReplaceBlock = true
			out = append(out, line)
			continue
		}
		if inReplaceBlock && trim == ")" {
			inReplaceBlock = false
			out = append(out, line)
			continue
		}

		if strings.Contains(trim, "=>") {
			var rest string
			switch {
			case inReplaceBlock:
				rest = trim
			default:
				// go.mod allows arbitrary whitespace after the `replace`
				// keyword (e.g. `replace\tfoo => bar`), so tokenize rather
				// than match a fixed "replace " prefix. Skip any other line
				// containing "=>" (e.g. an in-comment arrow).
				fields := strings.Fields(trim)
				if len(fields) == 0 || fields[0] != "replace" {
					out = append(out, line)
					continue
				}
				rest = strings.TrimSpace(strings.TrimPrefix(trim, fields[0]))
			}
			parts := strings.SplitN(rest, "=>", 2)
			from := strings.Fields(strings.TrimSpace(parts[0]))
			if len(from) >= 1 && conflicts[from[0]] {
				continue
			}
		}

		out = append(out, line)
	}
	return out
}

// formatReplaceLine renders a single replace directive. The "from" version
// is optional (e.g. `foo => ../foo`); the "to" version is absent for
// local-path replacements.
func formatReplaceLine(r []Package) string {
	if len(r) < 2 {
		return ""
	}
	from, to := r[0], r[1]

	left := from.Name
	if from.Version != "" {
		left = from.Name + " " + from.Version
	}

	right := to.Name
	if to.Version != "" {
		right = to.Name + " " + to.Version
	}
	return "\t" + left + " => " + right
}

// NewGoMod takes an io.Reader to a go.mod and returns GoMod struct
func New(f io.Reader) (*GoMod, error) {
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var g GoMod
	g.RequiredVersions = getRequiredVersionsFromString(string(b))
	g.ReplacedVersions = getReplacedVersionsFromString(string(b))
	return &g, nil
}

func getRequiredVersionsFromString(s string) (p []Package) {
	reqs := RequirePatternRegex.FindAllString(s, -1)
	for _, req := range reqs {
		data := getStringWithinCharacters(req, '(', ')')
		data = strings.TrimSuffix(strings.TrimPrefix(data, "\n"), "\n")
		packages := strings.Split(data, "\n")
		for _, pkg := range packages {
			pkg = strings.TrimSpace(pkg)
			if pkg == "" {
				continue
			}

			pkgName, pkgVersion := getPackageAndVersionFromPackageVersion(pkg)
			if strings.HasPrefix(pkgName, "//") { //Has been commented out
				continue
			}
			var indirect bool
			if strings.HasSuffix(pkgVersion, "//indirect") {
				pkgVersion = strings.TrimSuffix(pkgVersion, "//indirect")
				indirect = true
			}
			p = append(p, Package{
				Name:     pkgName,
				Version:  pkgVersion,
				Indirect: indirect,
			})
		}
	}
	return p
}
func getReplacedVersionsFromString(s string) (p [][]Package) {
	// Block form: replace ( ... )
	reps := ReplacePatternRegex.FindAllString(s, -1)
	for _, req := range reps {
		data := getStringWithinCharacters(req, '(', ')')
		data = strings.TrimSuffix(strings.TrimPrefix(data, "\n"), "\n")
		packages := strings.Split(data, "\n")
		for _, pkg := range packages {
			pkg = strings.TrimSpace(pkg)
			if pkg == "" {
				continue
			}
			p0 := getPackagesAndVersionsFromPackageVersions(pkg)
			if len(p0) != 0 {
				p = append(p, p0)
			}
		}
	}

	// Single-line form: `replace foo [v] => bar [v]` outside any block.
	// go.mod allows arbitrary whitespace after the `replace` keyword, so
	// tokenize rather than match a fixed prefix.
	inBlock := false
	for _, line := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(line)
		if !inBlock && (trim == "replace (" || trim == "replace(") {
			inBlock = true
			continue
		}
		if inBlock {
			if trim == ")" {
				inBlock = false
			}
			continue
		}
		if !strings.Contains(trim, "=>") {
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) == 0 || fields[0] != "replace" {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trim, fields[0]))
		p0 := getPackagesAndVersionsFromPackageVersions(rest)
		if len(p0) != 0 {
			p = append(p, p0)
		}
	}
	return p
}
func getPackagesAndVersionsFromPackageVersions(pkg string) (p []Package) {
	s := strings.Split(pkg, "=>")
	if len(s) < 2 {
		log.Fatal("invalid go mod")
	}
	pkg1, ver1 := getPackageAndVersionFromPackageVersion(s[0])
	pkg2, ver2 := getPackageAndVersionFromPackageVersion(s[1])
	if strings.HasPrefix(pkg1, "//") {
		return
	}
	p = append(p, Package{
		Name:    pkg1,
		Version: ver1,
	})
	p = append(p, Package{
		Name:    pkg2,
		Version: ver2,
	})

	return
}
func getStringWithinCharacters(s string, ch1 rune, ch2 rune) (s2 string) {
	i := 0
	for _, ch := range s {
		if ch == ch1 {
			i = 1
			continue
		}
		if ch == ch2 {
			i = 2
			continue
		}
		if i == 1 {
			s2 += string(ch)
		} else if i == 2 {
			break
		}
	}
	return
}

func getPackageAndVersionFromPackageVersion(pkgversion string) (pkg string, version string) {
	i := 0
	pkgversion = strings.TrimPrefix(pkgversion, " ")
	for _, s := range pkgversion {
		if s == ' ' && i == 0 { //first space
			i = 1
		} else if s != ' ' && i == 0 { //still parsing package name
			pkg += string(s)
		} else if s != ' ' && i == 1 {
			version += string(s) //now parsing version
		}
	}
	return
}
