package walker

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Git represents the Git Walker
type Git struct {
	baseURL            string
	owner              string
	repo               string
	branch             string
	root               string // If the root ends with "/**", then recurse is set to true
	recurse            bool
	showLogs           bool  // By default the logs of gitwalker are not displayed
	maxFileSizeInBytes int64 //defaults to 50 MB
	fileInterceptor    FileInterceptor
	dirInterceptor     DirInterceptor
	referenceName      plumbing.ReferenceName
}

// NewGit returns a pointer to an instance of Git
func NewGit() *Git {
	return &Git{
		branch:             "master",
		baseURL:            "https://github.com", //defaults to a github repo if the url is not set with URL method
		maxFileSizeInBytes: 50000000,             // ~50MB file size limit
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
	g.baseURL = baseurl
	return g
}

// BaseURL sets git repository base URL and returns a pointer
// to the same Git instance
func (g *Git) MaxFileSize(size int64) *Git {
	g.maxFileSizeInBytes = size
	return g
}

// ShowLogs enable the logs and returns a pointer
// to the same Git instance
func (g *Git) ShowLogs() *Git {
	g.showLogs = true
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
// If path will be prefixed with "/" if not already.
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

func (g *Git) ReferenceName(refName string) *Git {
	g.referenceName = plumbing.ReferenceName(refName)
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
	if g.maxFileSizeInBytes == 0 {
		return ErrInvalidSizeFile(errors.New("Max file size passed as 0. Will not read any file"))
	}

	path := filepath.Join(os.TempDir(), g.repo, strconv.FormatInt(time.Now().UTC().UnixNano(), 10))
	defer os.RemoveAll(path)
	var err error
	cloneOptions := &git.CloneOptions{
		URL:          fmt.Sprintf("%s/%s/%s", g.baseURL, g.owner, g.repo),
		SingleBranch: true,
	}

	if g.referenceName != "" {
		cloneOptions.ReferenceName = g.referenceName
	}

	if g.showLogs {
		cloneOptions.Progress = os.Stdout
		_, err = git.PlainClone(path, false, cloneOptions)
	} else {
		_, err = git.PlainClone(path, false, cloneOptions)
	}

	if err != nil {
		return ErrCloningRepo(err)
	}

	rootPath := filepath.Join(path, g.root)
	info, err := os.Stat(rootPath)
	if err != nil {
		return ErrCloningRepo(err)
	}

	if !info.IsDir() {
		err = g.readFile(info, rootPath)
		if err != nil {
			return ErrCloningRepo(err)
		}
		return nil
	}
	// If recurse mode is on, we will walk the tree
	if g.recurse {
		err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, er error) error {
			if d.IsDir() && g.dirInterceptor != nil {
				return g.dirInterceptor(Directory{
					Name: d.Name(),
					Path: path,
				})
			}
			if d.IsDir() {
				return nil
			}
			f, errInfo := d.Info()
			if err != nil {
				return errInfo
			}
			return g.readFile(f, path)
		})
		if err != nil {
			return ErrCloningRepo(err)
		}
		return nil
	}

	// If recurse mode is off, we only walk the root directory passed with g.root
	entries, err := os.ReadDir(filepath.Join(path, g.root))
	if err != nil {
		return err
	}
	files := make([]fs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		file, err := entry.Info()
		if err != nil {
			return ErrCloningRepo(err)
		}
		files = append(files, file)
	}

	for _, f := range files {
		fPath := filepath.Join(path, g.root, f.Name())
		if f.IsDir() && g.dirInterceptor != nil {
			name := f.Name()
			go func(name string, path string, filename string) {
				err := g.dirInterceptor(Directory{
					Name: filename,
					Path: fPath,
				})
				if err != nil {
					fmt.Println(err.Error())
				}
			}(name, fPath, f.Name())
			continue
		}
		if f.IsDir() {
			continue
		}
		err := g.readFile(f, fPath)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	return nil
}

func (g *Git) readFile(f fs.FileInfo, path string) error {
	if f.Size() > g.maxFileSizeInBytes {
		return ErrInvalidSizeFile(errors.New("File exceeding size limit"))
	}
	filename, err := os.Open(path)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(filename)
	if err != nil {
		return err
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
