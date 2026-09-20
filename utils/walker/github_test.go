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
	"time"
)

// githubContentsStub serves the Contents API endpoints the Github walker uses,
// so that no test reaches the real network.
type githubContentsStub struct {
	dirs  map[string]GithubDirectoryContentAPI
	files map[string]GithubContentAPI
	// forbidden, when set, is the message every request is refused with.
	forbidden string
	// onRequest, when set, runs before a request is answered.
	onRequest func(requested string, r *http.Request)

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

		if s.onRequest != nil {
			s.onRequest(requested, r)
		}

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

func TestGithubWalkContextFailsWhenCancellationTruncatesTheWalk(t *testing.T) {
	// The top-level listing is served in full, so the walk only breaks down in
	// the fan-out below it - the point at which a truncated import used to be
	// reported as a success.
	newStub := func() *githubContentsStub {
		return &githubContentsStub{
			dirs: map[string]GithubDirectoryContentAPI{
				"configs": {{Name: "child.yaml", Path: "configs/child.yaml", Type: "file"}},
			},
			files: map[string]GithubContentAPI{
				"configs/child.yaml": {Name: "child.yaml", Path: "configs/child.yaml", Type: "file", Encoding: "base64", Content: "a2luZDogQ29uZmlnTWFw"},
			},
		}
	}

	t.Run("a walk cancelled after the listing fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		stub := newStub()
		stub.onRequest = func(requested string, r *http.Request) {
			if requested != "configs/child.yaml" {
				return
			}
			// Cancelled while this child request is in flight, and held until
			// the client has given up on it, so the walk is always truncated.
			cancel()
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		}
		server := stub.server(t)

		var mu sync.Mutex
		listings := 0
		err := contentsGithub(server).
			Root("configs").
			RegisterFileInterceptor(func(GithubContentAPI) error { return nil }).
			RegisterDirInterceptor(func(GithubDirectoryContentAPI) error {
				mu.Lock()
				defer mu.Unlock()
				listings++
				return nil
			}).
			WalkContext(ctx)
		if err == nil {
			t.Fatal("expected a walk truncated by cancellation to fail rather than report success")
		}

		mu.Lock()
		defer mu.Unlock()
		if listings != 0 {
			t.Errorf("expected no directory listing to be handed over by a truncated walk, got %d", listings)
		}
	})

	t.Run("an uncancelled walk still succeeds", func(t *testing.T) {
		server := newStub().server(t)

		var mu sync.Mutex
		intercepted := []string{}
		err := contentsGithub(server).
			Root("configs").
			RegisterFileInterceptor(func(file GithubContentAPI) error {
				mu.Lock()
				defer mu.Unlock()
				intercepted = append(intercepted, file.Path)
				return nil
			}).
			WalkContext(context.Background())
		if err != nil {
			t.Fatalf("WalkContext() returned error: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()
		if !reflect.DeepEqual(intercepted, []string{"configs/child.yaml"}) {
			t.Errorf("expected the file to be delivered, got %v", intercepted)
		}
	})
}

func TestGithubWalkContextKeepsWalkingPastANodeItCannotDecode(t *testing.T) {
	// GitHub answers a submodule or a symlink path with a single JSON object
	// rather than a listing, so the walker cannot decode it. Every other file
	// under the root still has to arrive.
	stub := &githubContentsStub{
		dirs: map[string]GithubDirectoryContentAPI{
			"configs": {
				{Name: "child.yaml", Path: "configs/child.yaml", Type: "file"},
				{Name: "vendor", Path: "configs/vendor", Type: "submodule"},
			},
		},
		files: map[string]GithubContentAPI{
			"configs/child.yaml": {Name: "child.yaml", Path: "configs/child.yaml", Type: "file", Encoding: "base64", Content: "a2luZDogQ29uZmlnTWFw"},
			"configs/vendor":     {Name: "vendor", Path: "configs/vendor", Type: "submodule"},
		},
	}
	server := stub.server(t)

	var mu sync.Mutex
	intercepted := []string{}
	listings := 0
	err := contentsGithub(server).
		Root("configs/**").
		RegisterFileInterceptor(func(file GithubContentAPI) error {
			mu.Lock()
			defer mu.Unlock()
			intercepted = append(intercepted, file.Path)
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
		t.Fatalf("expected a node that cannot be decoded to be passed over, got error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(intercepted, []string{"configs/child.yaml"}) {
		t.Errorf("expected the readable file to be delivered, got %v", intercepted)
	}
	if listings != 1 {
		t.Errorf("expected the directory listing to still be handed over, got %d", listings)
	}
}

func TestGithubWalkSendsTheBranchAndPathIntact(t *testing.T) {
	// "#" is a character git accepts in a reference name, and it ends the URL
	// at the fragment when it travels raw, so the request would ask for a
	// different branch than the caller configured.
	stub := &githubContentsStub{
		files: map[string]GithubContentAPI{
			"configs#1/child.yaml": {Name: "child.yaml", Path: "configs#1/child.yaml", Type: "file"},
		},
	}
	var target string
	stub.onRequest = func(_ string, r *http.Request) {
		if target == "" {
			target = r.URL.RequestURI()
		}
	}
	server := stub.server(t)

	_ = contentsGithub(server).
		Branch("feat#1").
		Root("configs#1/child.yaml").
		RegisterFileInterceptor(func(GithubContentAPI) error { return nil }).
		WalkContext(context.Background())

	want := "/repos/owner/repo/contents/configs%231/child.yaml?ref=feat%231"
	if target != want {
		t.Errorf("expected the request target %q, got %q", want, target)
	}
}
