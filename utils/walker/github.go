package walker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
)

type Mode string

const (
	WalkTheRepo  = "WALK_THE_REPO"
	CloneAndWalk = "CLONE_AND_WALK"
)

var modeToFileIntercepterName = map[Mode]string{
	WalkTheRepo:  "RegisterFileInterceptor",
	CloneAndWalk: "RegisterLocalFileInterceptor",
}
var modeToDirIntercepterName = map[Mode]string{
	WalkTheRepo:  "RegisterDirInterceptor",
	CloneAndWalk: "RegisterLocalDirInterceptor",
}

// GithubContentAPI represents Github API v3 response
// to /repos/{owner}/{repo}/contents/{path}?ref={branch}
type GithubContentAPI struct {
	Name        string `json:"name,omitempty"`
	Path        string `json:"path,omitempty"`
	SHA         string `json:"sha,omitempty"`
	Size        int64  `json:"size,omitempty"`
	URL         string `json:"url,omitempty"`
	HTMLURL     string `json:"html_url,omitempty"`
	GitURL      string `json:"git_url,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	Type        string `json:"type,omitempty"`
	// Content will be empty when the path is of a directory
	Content string `json:"content,omitempty"`
	// Encoding will be empty when the path is of a directory
	Encoding string `json:"encoding,omitempty"`
}

// GithubDirectoryContentAPI represents Github API v3 response
// to /repos/{owner}/{repo}/contents/{path}?ref={branch} when "path"
// is of a directory
type GithubDirectoryContentAPI []GithubContentAPI

// GithubFileInterceptor represents function signature which
// will be used on "file" nodes when the github walker traverses
// the paths
type GithubFileInterceptor func(GithubContentAPI) error

// GithubDirInterceptor represents function signature which
// will be used on "dir" nodes when the github walker traverses
// the paths
type GithubDirInterceptor func(GithubDirectoryContentAPI) error

// Github represents the Github Walker
type Github struct {
	owner              string
	repo               string
	branch             string
	root               string
	recurse            bool
	mode               Mode
	gitfileInterceptor GithubFileInterceptor
	gitdirInterceptor  GithubDirInterceptor

	fileInterceptor FileInterceptor
	dirInterceptor  DirInterceptor
}

// NewGithub returns a pointer to an instance of Github
func NewGithub() *Github {
	return &Github{
		branch: "main",
	}
}

// Owner sets github repository owner and returns a pointer
// to the same Github instance
func (g *Github) Owner(owner string) *Github {
	g.owner = owner
	return g
}

// Repo sets github repository and returns a pointer
// to the same Github instance
func (g *Github) Repo(repo string) *Github {
	g.repo = repo
	return g
}

// Branch sets github repository branch which
// will be traversed and returns a pointer
// to the same Github instance
func (g *Github) Branch(branch string) *Github {
	g.branch = branch
	return g
}

// Root sets github repository root node from where
// Github walker needs to start traversing and returns
// a pointer to the same Github instance
//
// If the root parameter ends with a "/**" then github walker
// will run in "traversal" mode, ie. it will look into each sub
// directory of the root node
//
// If the root node ends with an extension, then that
// file will be returned and github walker will not traverse deeper
func (g *Github) Root(root string) *Github {
	g.root = root

	if strings.HasSuffix(root, "/**") {
		g.recurse = true
		g.root = strings.TrimSuffix(root, "/**")
	}

	return g
}

func (g *Github) Mode(mode Mode) *Github {
	g.mode = mode
	return g
}

// If you change the name of registeration functions, reflect them in the maps above
// RegisterFileInterceptor takes in a file interceptor which will be invoked
// on each "file" node and it returns pointer to the same github instance
//
// Github Walker walks the nodes concurrently so if the interceptor is reading
// or writing to any variable from a higher namespace then those operations
// should be done in thread safe manner in order to avoid data races
func (g *Github) RegisterFileInterceptor(i GithubFileInterceptor) *Github {
	if g.mode != WalkTheRepo {
		log.Fatalf("Invalid register function for mode %s, use %s instead", g.mode, getFileFuncName(g.mode))
		return g
	}
	g.gitfileInterceptor = i
	return g
}
func (g *Github) RegisterLocalFileInterceptor(i FileInterceptor) *Github {
	if g.mode != CloneAndWalk {
		log.Fatalf("Invalid register function for mode %s, use %s instead", g.mode, getFileFuncName(g.mode))
		return g
	}
	g.fileInterceptor = i
	return g
}

// RegisterFileInterceptor takes in a directory interceptor which will be invoked
// on each "directory" node and it returns pointer to the same github instance
//
// Github Walker walks the nodes concurrently so if the interceptor is reading
// or writing to any variable from a higher namespace then those operations
// should be done in thread safe manner in order to avoid data races
func (g *Github) RegisterDirInterceptor(i GithubDirInterceptor) *Github {
	if g.mode != WalkTheRepo {
		log.Fatalf("Invalid register function for mode %s, use %s instead", g.mode, getDirFuncName(g.mode))
		return g
	}
	g.gitdirInterceptor = i
	return g
}

func (g *Github) RegisterLocalDirInterceptor(i DirInterceptor) *Github {
	if g.mode != CloneAndWalk {
		log.Fatalf("Invalid register function for mode %s, use %s instead", g.mode, getDirFuncName(g.mode))
		return g
	}
	g.dirInterceptor = i
	return g
}

// Walk will initiate traversal process
func (g *Github) Walk() error {
	switch g.mode {
	case CloneAndWalk:
		return clonewalk(g)
	case WalkTheRepo:
		return repowalk(g)
	default:
		return ErrInvalidCode(errors.New("INVALID MODE"))
	}

}

func repowalk(g *Github) error {
	// Check if a file is requested
	isFile := true
	if g.root == "" || filepath.Ext(g.root) == "" {
		isFile = false
	}

	return g.walker(g.root, isFile)
}
func clonewalk(g *Github) error {
	_, err := git.PlainClone(Path, false, &git.CloneOptions{
		URL:      fmt.Sprintf("https://github.com/%s/%s", g.owner, g.repo),
		Progress: os.Stdout,
	})
	if err != nil {
		return ErrCloningRepo(err)
	}
	if g.recurse {
		err := filepath.WalkDir(Path+g.root, func(path string, d fs.DirEntry, err error) error {
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
	files, err := ioutil.ReadDir(Path + g.root)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		path := Path + g.root + "/" + f.Name()
		if f.IsDir() && g.dirInterceptor != nil {
			name := f.Name()
			go func(name string, path string) error {
				return g.dirInterceptor(Directory{
					Name: f.Name(),
					Path: path,
				})
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

// walker is a recursive function which actually walks the Github tree
func (g *Github) walker(path string, isFile bool) error {
	githubAPIURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		g.owner,
		g.repo,
		path,
		g.branch,
	)

	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			respJSON := map[string]interface{}{}

			if err := json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
				return fmt.Errorf("[GithubWalker]: GithubAPI responded with: forbidden")
			}

			message, ok := respJSON["message"].(string)
			if !ok {
				return fmt.Errorf("[GithubWalker]: GithubAPI responded with: forbidden")
			}

			return fmt.Errorf(message)
		}
		return fmt.Errorf("file not found")
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error("failed to close response body", err)
		}
	}()

	if isFile {
		respBody := GithubContentAPI{}

		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			logrus.Error("[GithubWalker]: failed to decode API response", err)
			return err
		}

		if g.gitfileInterceptor != nil {
			return g.gitfileInterceptor(respBody)
		}

		return nil
	}

	respBody := GithubDirectoryContentAPI{}

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		logrus.Error("[GithubWalker]: failed to decode API response", err)
		return err
	}

	var wg sync.WaitGroup
	for _, r := range respBody {
		nextPath := r.Path
		typ := r.Type
		isFile := typ == "file"

		wg.Add(1)
		go func(r GithubContentAPI) {
			if g.recurse || isFile {
				if err := g.walker(nextPath, isFile); err != nil {
					logrus.Error("[GithubWalker]: error occurred while processing github node ", err)
				}
			}

			wg.Done()
		}(r)
	}

	wg.Wait()

	if g.gitdirInterceptor != nil {
		if err := g.gitdirInterceptor(respBody); err != nil {
			logrus.Error("[GithubWalker]: error occurred while executing directory interceptor function ", err)
			return err
		}
	}

	return nil
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

const Path = "./test"

func getFileFuncName(mode Mode) string {
	name := modeToFileIntercepterName[mode]
	if name == "" {
		return "NO FUNCTION PRESENT: [INVALID OR EMPTY MODE PASSED]"
	}
	return name
}
func getDirFuncName(mode Mode) string {
	name := modeToDirIntercepterName[mode]
	if name == "" {
		return "NO FUNCTION PRESENT: [INVALID OR EMPTY MODE PASSED]"
	}
	return name
}
