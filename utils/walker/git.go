package walker

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
)

// Git represents the Git Walker
type Git struct {
	baseurl            string
	owner              string
	repo               string
	branch             string
	root               string
	recurse            bool
	showlogs           bool
	maxfilesizeinbytes int64 //defaults to 50 MB
	fileInterceptor    FileInterceptor
	dirInterceptor     DirInterceptor
}

// NewGit returns a pointer to an instance of Git
func NewGit() *Git {
	return &Git{
		branch:             "master",
		baseurl:            "https://github.com", //defaults to a github repo if the url is not set with URL method
		maxfilesizeinbytes: 50000000,
	}
}

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

// BaseURL sets git repository base URL and returns a pointer
// to the same Git instance
func (g *Git) BaseURL(baseurl string) *Git {
	g.baseurl = baseurl
	return g
}

// BaseURL sets git repository base URL and returns a pointer
// to the same Git instance
func (g *Git) MaxFileSize(size int64) *Git {
	g.maxfilesizeinbytes = size
	return g
}

// ShowLogs enable the logs and returns a pointer
// to the same Git instance
func (g *Git) ShowLogs() *Git {
	g.showlogs = true
	return g
}

// Owner sets git repository owner and returns a pointer
// to the same Git instance
func (g *Git) Owner(owner string) *Git {
	g.owner = owner
	return g
}

// Repo sets github repository and returns a pointer
// to the same Git instance
func (g *Git) Repo(repo string) *Git {
	g.repo = repo
	return g
}

// Branch sets git repository branch which
// will be cloned and returns a pointer
// to the same Git instance
func (g *Git) Branch(branch string) *Git {
	g.branch = branch
	return g
}

// Root sets git repository root node from where
// Git walker needs to start traversing and returns
// a pointer to the same Git instance
//
// If the root parameter ends with a "/**" then github walker
// will run in "traversal" mode, ie. it will look into each sub
// directory of the root node
//If path will be prefixed with "/" if not already.
func (g *Git) Root(root string) *Git {
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	g.root = root

	if strings.HasSuffix(root, "/**") {
		g.recurse = true
		g.root = strings.TrimSuffix(root, "/**")
	}

	return g
}

// Walk will initiate traversal process
func (g *Git) Walk() error {
	return clonewalk(g)
}
func (g *Git) RegisterFileInterceptor(i FileInterceptor) *Git {
	g.fileInterceptor = i
	return g
}

func (g *Git) RegisterDirInterceptor(i DirInterceptor) *Git {
	g.dirInterceptor = i
	return g
}
func clonewalk(g *Git) error {
	if g.maxfilesizeinbytes == 0 {
		return ErrInvalidSizeFile(errors.New("Max file size passed as 0. Will not read any file"))
	}

	path := filepath.Join(os.TempDir(), g.repo, strconv.FormatInt(time.Now().UTC().UnixNano(), 10))
	os.RemoveAll(path) //In case repo by same name already exists in temp
	defer os.RemoveAll(path)
	logs := os.Stdout
	if !g.showlogs {
		logs = nil
	}
	_, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:      fmt.Sprintf("%s/%s/%s", g.baseurl, g.owner, g.repo),
		Progress: logs,
	})
	if err != nil {
		return ErrCloningRepo(err)
	}

	// If recurse mode is on, we will walk the tree
	if g.recurse {
		err := filepath.WalkDir(filepath.Join(path, g.root), func(path string, d fs.DirEntry, er error) error {
			if d.IsDir() && g.dirInterceptor != nil {
				return g.dirInterceptor(Directory{
					Name: d.Name(),
					Path: path,
				})
			}
			if d.IsDir() {
				return nil
			}
			f, err := d.Info()
			if err != nil {
				return err
			}
			return g.readFile(f, path)
		})
		return err
	}

	// If recurse mode is off, we only walk the root directory passed with g.root
	files, err := ioutil.ReadDir(filepath.Join(path, g.root))
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {

		path := filepath.Join(path, g.root, f.Name())
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
		g.readFile(f, path)
	}

	return nil
}

func (g *Git) readFile(f fs.FileInfo, path string) error {
	if f.Size() > g.maxfilesizeinbytes {
		return ErrInvalidSizeFile(errors.New("File exceeding size limit"))
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
		fmt.Println("Could not intercept the file ", f.Name())
	}
	return err
}
