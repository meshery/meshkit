package walker

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"errors"

	"github.com/go-git/go-git/v5"
)

type Mode string

const (
	WalkTheRepo  = "WALK_THE_REPO"
	CloneAndWalk = "CLONE_AND_WALK"
)

type File struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
	Path    string `json:"path,omitempty"`
}
type Directory struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}
type FileInterceptor func(File) error
type DirInterceptor func(Directory) error

var modeToFileIntercepterName = map[Mode]string{
	WalkTheRepo:  "RegisterFileInterceptor",
	CloneAndWalk: "RegisterLocalFileInterceptor",
}
var modeToDirIntercepterName = map[Mode]string{
	WalkTheRepo:  "RegisterDirInterceptor",
	CloneAndWalk: "RegisterLocalDirInterceptor",
}

// Walk will initiate traversal process
func (g *Github) Walk() error {
	switch g.mode {
	case CloneAndWalk:
		return clonewalk(g)
	case WalkTheRepo:
		return repowalk(g)
	default:
		return ErrInvalidMode(errors.New("Invalid mode"))
	}
}
func (g *Github) RegisterLocalFileInterceptor(i FileInterceptor) *Github {
	funcName, err := GetFileFuncName(g.mode)
	if err != nil {
		g.mode = ""
		return g
	}
	if g.mode != CloneAndWalk {
		log.Fatalf("Invalid register function for mode %s, use %s instead", g.mode, funcName)
		g.mode = ""
		return g
	}
	g.fileInterceptor = i
	return g
}

func (g *Github) RegisterLocalDirInterceptor(i DirInterceptor) *Github {
	funcName, err := GetDirFuncName(g.mode)
	if err != nil {
		g.mode = ""
		return g
	}
	if g.mode != CloneAndWalk {
		log.Fatalf("Invalid register function for mode %s, use %s instead", g.mode, funcName)
		g.mode = ""
		return g
	}
	g.dirInterceptor = i
	return g
}
func clonewalk(g *Github) error {
	os.RemoveAll(os.TempDir() + "/" + g.repo) //In case repo by same name already exists in temp
	defer os.RemoveAll(os.TempDir() + "/" + g.repo)
	path := os.TempDir() + "/" + g.repo
	_, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:      fmt.Sprintf("https://github.com/%s/%s", g.owner, g.repo),
		Progress: os.Stdout,
	})
	if err != nil {
		return ErrCloningRepo(err)
	}

	// If recurse mode is on, we will walk the tree
	if g.recurse {
		err := filepath.WalkDir(path+g.root, func(path string, d fs.DirEntry, er error) error {
			if d.IsDir() && g.dirInterceptor != nil {
				return g.dirInterceptor(Directory{
					Name: d.Name(),
					Path: path,
				})
			}
			if d.IsDir() {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			content, err := ioutil.ReadAll(file)
			if err != nil {
				return err
			}
			if g.fileInterceptor == nil {
				return nil
			}
			return g.fileInterceptor(File{
				Name:    d.Name(),
				Content: string(content),
				Path:    path,
			})
		})
		return err
	}

	// If recurse mode is off, we only walk the root directory passed with g.root
	files, err := ioutil.ReadDir(path + g.root)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		path := path + g.root + "/" + f.Name()
		if f.IsDir() && g.dirInterceptor != nil {
			name := f.Name()
			go func(name string, path string) {
				err := g.dirInterceptor(Directory{
					Name: f.Name(),
					Path: path,
				})
				if err != nil {
					log.Fatal(err)
				}
			}(name, path)
			continue
		}
		if f.IsDir() {
			return nil
		}
		filename, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		content, err := ioutil.ReadAll(filename)
		if err != nil {
			log.Fatal(err)
		}
		err = g.fileInterceptor(File{
			Name:    f.Name(),
			Path:    path,
			Content: string(content),
		})
		if err != nil {
			log.Fatal("Could not intercept the file ", f.Name())
		}
	}

	return nil
}

func GetFileFuncName(mode Mode) (string, error) {
	name := modeToFileIntercepterName[mode]
	if name == "" {
		return "", ErrInvalidMode(errors.New("No registeration function present for this mode"))
	}
	return name, nil
}
func GetDirFuncName(mode Mode) (string, error) {
	name := modeToDirIntercepterName[mode]
	if name == "" {
		return "", ErrInvalidMode(errors.New("No registeration function present for this mode"))
	}
	return name, nil
}
