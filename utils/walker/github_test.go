package walker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// githubContentsStub serves the Contents API endpoints the Github walker uses,
// so that no test reaches the real network.
type githubContentsStub struct {
	dirs  map[string]GithubDirectoryContentAPI
	files map[string]GithubContentAPI
	// forbidden, when set, is the message every request is refused with.
	forbidden string

	mu            sync.Mutex
	paths         []string
	refs          []string
	authorization []string
}

func (s *githubContentsStub) server(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo/contents/")

		s.mu.Lock()
		s.paths = append(s.paths, requested)
		s.refs = append(s.refs, r.URL.Query().Get("ref"))
		s.authorization = append(s.authorization, r.Header.Get("Authorization"))
		s.mu.Unlock()

		if s.forbidden != "" {
			w.WriteHeader(http.StatusForbidden)
			writeJSON(t, w, map[string]string{"message": s.forbidden})
			return
		}
		if listing, ok := s.dirs[requested]; ok {
			writeJSON(t, w, listing)
			return
		}
		if file, ok := s.files[requested]; ok {
			writeJSON(t, w, file)
			return
		}

		w.WriteHeader(http.StatusNotFound)
		writeJSON(t, w, map[string]string{"message": "Not Found"})
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *githubContentsStub) snapshot() (paths, refs, auth []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...), append([]string(nil), s.refs...), append([]string(nil), s.authorization...)
}

// contentsGithub returns a Contents walker for owner/repo pointed at the stub
// server. The API endpoint is not part of the published API, so the tests set
// the field.
func contentsGithub(server *httptest.Server) *Github {
	g := NewGithub().Owner("owner").Repo("repo")
	g.apiBaseURL = server.URL
	return g
}

func TestGithubWalkContextAuthenticatesEveryRequestAndReportsProgress(t *testing.T) {
	stub := &githubContentsStub{
		dirs: map[string]GithubDirectoryContentAPI{
			"configs": {{Name: "child.yaml", Path: "configs/child.yaml", Type: "file"}},
		},
		files: map[string]GithubContentAPI{
			"configs/child.yaml": {Name: "child.yaml", Path: "configs/child.yaml", Type: "file", Encoding: "base64", Content: "a2luZDogQ29uZmlnTWFw"},
		},
	}
	server := stub.server(t)

	// The walker fans out one goroutine per entry, so everything the hook and
	// the interceptors collect is guarded.
	var mu sync.Mutex
	stages := []ProgressStage{}
	progressed := []string{}
	intercepted := []GithubContentAPI{}
	listings := 0

	err := contentsGithub(server).
		Branch("release").
		Root("configs").
		Token("s3cret").
		RegisterProgressHook(func(update ProgressUpdate) {
			mu.Lock()
			defer mu.Unlock()
			stages = append(stages, update.Stage)
			progressed = append(progressed, update.Message)
		}).
		RegisterFileInterceptor(func(file GithubContentAPI) error {
			mu.Lock()
			defer mu.Unlock()
			intercepted = append(intercepted, file)
			return nil
		}).
		RegisterDirInterceptor(func(GithubDirectoryContentAPI) error {
			mu.Lock()
			defer mu.Unlock()
			listings++
			return nil
		}).
		WalkContext(context.Background())
	if err != nil {
		t.Fatalf("WalkContext() returned error: %v", err)
	}

	if len(intercepted) != 1 || intercepted[0].Path != "configs/child.yaml" {
		t.Fatalf("expected the file entry to be intercepted, got %+v", intercepted)
	}
	if intercepted[0].Content != "a2luZDogQ29uZmlnTWFw" {
		t.Errorf("expected the API's content to be handed over untouched, got %q", intercepted[0].Content)
	}
	if listings != 1 {
		t.Errorf("expected the directory listing to be intercepted once, got %d", listings)
	}

	paths, refs, auth := stub.snapshot()
	if len(auth) != 2 {
		t.Fatalf("expected the directory and the file to be requested, got %v", paths)
	}
	for _, header := range auth {
		if header != "Bearer s3cret" {
			t.Errorf("expected every request to carry the bearer token, got %q", header)
		}
	}
	for _, ref := range refs {
		if ref != "release" {
			t.Errorf("expected every request to ask for the configured branch, got %q", ref)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for _, stage := range stages {
		if stage != ProgressStageListTree {
			t.Errorf("expected the walk to report %q, got %q", ProgressStageListTree, stage)
		}
	}
	sort.Strings(progressed)
	if want := []string{"configs", "configs/child.yaml"}; !reflect.DeepEqual(progressed, want) {
		t.Errorf("expected progress for %v, got %v", want, progressed)
	}
}

func TestGithubWalkContextStopsOnACancelledContext(t *testing.T) {
	stub := &githubContentsStub{
		dirs: map[string]GithubDirectoryContentAPI{
			"configs": {{Name: "child.yaml", Path: "configs/child.yaml", Type: "file"}},
		},
	}
	server := stub.server(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := contentsGithub(server).
		Root("configs").
		RegisterFileInterceptor(func(GithubContentAPI) error { return nil }).
		WalkContext(ctx)
	if err == nil {
		t.Fatal("expected WalkContext to fail once the context is cancelled")
	}

	paths, _, _ := stub.snapshot()
	if len(paths) != 0 {
		t.Errorf("expected no request to be issued under a cancelled context, got %v", paths)
	}
}

func TestGithubWalkContextSurfacesForbiddenWithoutTheToken(t *testing.T) {
	stub := &githubContentsStub{forbidden: "API rate limit exceeded"}
	server := stub.server(t)

	err := contentsGithub(server).
		Root("configs").
		Token("s3cret").
		RegisterFileInterceptor(func(GithubContentAPI) error { return nil }).
		WalkContext(context.Background())
	if err == nil {
		t.Fatal("expected WalkContext to fail when the API refuses the request")
	}
	if !strings.Contains(err.Error(), "API rate limit exceeded") {
		t.Errorf("expected the API's message to be surfaced, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatal("the access token must never appear in an error message")
	}
}
