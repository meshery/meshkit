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
	if len(g.ReplacedVersions) != 0 {
		data = append(data, "replace(")
	}

	//Add all the replaced versions from source to destination. Running go mod tidy after the utility will perform the cleanup in the destination go.mod and remove all the unwanted replace statements.
	//Instead of trying to intelligently perform diffs, it is better to let the go mod tidy do the cleanup.
	for _, replaced := range g.ReplacedVersions {
		data = append(data, fmt.Sprintf("\t%s %s => %s %s", replaced[0].Name, replaced[0].Version, replaced[1].Name, replaced[1].Version))
	}
	if len(g.ReplacedVersions) != 0 {
		data = append(data, ")")
	}
	gomod = strings.Join(data, "\n")
	return
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
