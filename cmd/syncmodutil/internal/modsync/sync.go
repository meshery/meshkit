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

	// Emit a single, canonical replace block that pins every package in the
	// source (host) module graph to its exact source-selected version.
	// Pinning via `replace` — rather than relying on `require` alone — is
	// required for Go plugin ABI compatibility. Without it, the `go mod tidy`
	// step that callers run after this tool can upgrade transitive
	// dependencies past what the host binary is linked against (for example,
	// a direct dependency on github.com/99designs/gqlgen pulling in a newer
	// github.com/vektah/gqlparser/v2 than the host uses), causing plugin.Open
	// to fail at runtime with:
	//   "plugin was built with a different version of package <pkg>".
	//
	// The block is assembled by merging three sources, highest precedence
	// first, keeping exactly one directive per module:
	//   1. Source-declared replaces (emitted verbatim). These override
	//      require resolution, so the destination compiles against the exact
	//      same versions as the source.
	//   2. Source require pins (module => module version).
	//   3. Destination-only replaces the source does not touch, so
	//      extension-specific overrides (e.g. a local `=> ../../meshery`
	//      path) survive.
	//
	// Every pre-existing replace in the destination is stripped first and the
	// merged set is re-emitted as one block. Emitting exactly one directive
	// per module is what makes the tool idempotent and guarantees the output
	// can never carry a duplicate or conflicting replacement — even when the
	// destination arrives already corrupted by a prior run (two conflicting
	// replaces for the same module in separate blocks, which makes the
	// caller's `go mod tidy` fail with "conflicting replacements for
	// <module>" before this tool can run again to heal it).
	final := make(map[string]string, len(g.RequiredVersions)+len(g.ReplacedVersions))
	order := make([]string, 0, len(g.RequiredVersions)+len(g.ReplacedVersions))
	addReplace := func(name, line string) {
		if name == "" || line == "" {
			return
		}
		if _, exists := final[name]; exists {
			return
		}
		final[name] = line
		order = append(order, name)
	}

	// 1. Source-declared replaces take precedence and are emitted verbatim.
	for _, replaced := range g.ReplacedVersions {
		if len(replaced) >= 1 {
			addReplace(replaced[0].Name, formatReplaceLine(replaced))
		}
	}
	// 2. Pin every source require to its exact version.
	for _, required := range g.RequiredVersions {
		addReplace(required.Name, fmt.Sprintf("\t%s => %s %s", required.Name, required.Name, required.Version))
	}
	// 3. Preserve destination-only replaces (deduplicated; first wins).
	for _, replaced := range getReplacedVersionsFromString(string(b)) {
		if len(replaced) >= 1 {
			addReplace(replaced[0].Name, formatReplaceLine(replaced))
		}
	}

	data = stripAllReplaces(data)

	if len(order) > 0 {
		data = append(data, "replace (")
		for _, name := range order {
			data = append(data, final[name])
		}
		data = append(data, ")")
	}
	gomod = strings.Join(data, "\n")
	return
}

// stripAllReplaces removes every replace directive from the destination
// go.mod lines — both block form (`replace ( ... )`, including the wrapping
// lines) and single-line form (`replace foo => bar v1`). The caller re-emits
// a single canonical, deduplicated replace block. Removing all pre-existing
// replaces (rather than only those being re-emitted) is what guarantees the
// result can never carry a duplicate or conflicting replacement, even when
// the destination arrived corrupted from a prior run. Any blank lines left
// where blocks were removed are normalized by the caller's `go mod tidy`.
func stripAllReplaces(data []string) []string {
	out := make([]string, 0, len(data))
	inReplaceBlock := false
	for _, line := range data {
		trim := strings.TrimSpace(line)

		if !inReplaceBlock && (trim == "replace (" || trim == "replace(") {
			inReplaceBlock = true
			continue // drop the opening line
		}
		if inReplaceBlock {
			if trim == ")" {
				inReplaceBlock = false
			}
			continue // drop block-interior lines and the closing ")"
		}

		// Single-line replace outside any block. go.mod allows arbitrary
		// whitespace after the `replace` keyword, so tokenize rather than
		// match a fixed prefix. Lines that merely contain "=>" elsewhere
		// (e.g. an in-comment arrow) are left untouched.
		if strings.Contains(trim, "=>") {
			fields := strings.Fields(trim)
			if len(fields) > 0 && fields[0] == "replace" {
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
