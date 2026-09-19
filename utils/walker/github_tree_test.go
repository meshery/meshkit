package walker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/schemas/models/core"
)

// githubAPIStub serves the three GitHub endpoints the hybrid crawl uses, so
// that no test reaches the real network.
type githubAPIStub struct {
	commitSHA string
	tree      githubTreeAPI
	blobs     map[string]string // blob SHA -> decoded content
	// onBlob, when set, runs before a blob response is written.
	onBlob func(sha string)

	mu            sync.Mutex
	received      []string
	authorization []string
	requestedRefs []string
	commitURIs    []string
	fetchedBlobs  []string
	repoLookups   int
}

func (s *githubAPIStub) server(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.repoLookups++
		s.mu.Unlock()
		s.record(r, nil, "")
		w.WriteHeader(http.StatusNotFound)
		writeJSON(t, w, map[string]string{"message": "the walker must resolve HEAD rather than look the repository up"})
	})
	mux.HandleFunc("/repos/owner/repo/commits/", func(w http.ResponseWriter, r *http.Request) {
		s.record(r, &s.commitURIs, r.RequestURI)
		s.record(r, &s.requestedRefs, strings.TrimPrefix(r.URL.Path, "/repos/owner/repo/commits/"))
		writeJSON(t, w, githubCommitAPI{SHA: s.commitSHA})
	})
	mux.HandleFunc("/repos/owner/repo/git/trees/", func(w http.ResponseWriter, r *http.Request) {
		s.record(r, nil, "")
		if got := r.URL.Query().Get("recursive"); got != "1" {
			t.Errorf("expected a recursive tree request, got recursive=%q", got)
		}
		writeJSON(t, w, s.tree)
	})
	mux.HandleFunc("/repos/owner/repo/git/blobs/", func(w http.ResponseWriter, r *http.Request) {
		sha := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo/git/blobs/")
		s.record(r, &s.fetchedBlobs, sha)

		if s.onBlob != nil {
			s.onBlob(sha)
		}

		content, ok := s.blobs[sha]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(t, w, map[string]string{"message": "Not Found"})
			return
		}
		writeJSON(t, w, githubBlobAPI{
			SHA:      sha,
			Size:     int64(len(content)),
			Encoding: "base64",
			// GitHub wraps base64 content, so wrap it here too.
			Content: base64.StdEncoding.EncodeToString([]byte(content))[:1] + "\n" + base64.StdEncoding.EncodeToString([]byte(content))[1:],
		})
	})

	// Recorded ahead of the mux, which cleans a path before a handler sees it,
	// so a request the walker should never have issued is still counted.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.received = append(s.received, r.RequestURI)
		s.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

// receivedRequests returns every request target the server was sent, including
// ones the mux answers with a redirect before any handler runs.
func (s *githubAPIStub) receivedRequests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.received...)
}

func (s *githubAPIStub) record(r *http.Request, sink *[]string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorization = append(s.authorization, r.Header.Get("Authorization"))
	if sink != nil {
		*sink = append(*sink, value)
	}
}

func (s *githubAPIStub) snapshot() (auth, refs, blobs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.authorization...), append([]string(nil), s.requestedRefs...), append([]string(nil), s.fetchedBlobs...)
}

// requestedCommitURIs returns the request targets the commits endpoint saw, as
// they arrived rather than as Go decodes them.
func (s *githubAPIStub) requestedCommitURIs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commitURIs...)
}

func (s *githubAPIStub) repoLookupCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repoLookups
}

// apiGit returns a walker for owner/repo pointed at the stub server. The API
// endpoint is not part of the published API, so the tests set the field.
func apiGit(server *httptest.Server) *Git {
	g := NewGit().Owner("owner").Repo("repo")
	g.apiBaseURL = server.URL
	return g
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Errorf("failed to write stub response: %v", err)
	}
}

func TestClassifyPath(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		wantKind        core.IaCFileTypes
		wantScore       int
		wantInteresting bool
	}{
		{name: "helm chart definition", path: "charts/redis/Chart.yaml", wantKind: core.HelmChart, wantScore: ScoreHelmChartDefinition, wantInteresting: true},
		{name: "helm chart definition yml", path: "Chart.yml", wantKind: core.HelmChart, wantScore: ScoreHelmChartDefinition, wantInteresting: true},
		{name: "kustomization", path: "overlays/prod/kustomization.yaml", wantKind: core.K8sKustomize, wantScore: ScoreKustomization, wantInteresting: true},
		{name: "kustomization yml", path: "overlays/prod/kustomization.yml", wantKind: core.K8sKustomize, wantScore: ScoreKustomization, wantInteresting: true},
		{name: "kustomization archive is not a kustomization", path: "overlays/kustomization.zip", wantKind: "", wantScore: 0, wantInteresting: false},
		{name: "docker compose", path: "docker-compose.yml", wantKind: core.DockerCompose, wantScore: ScoreDockerCompose, wantInteresting: true},
		{name: "compose", path: "deploy/compose.yaml", wantKind: core.DockerCompose, wantScore: ScoreDockerCompose, wantInteresting: true},
		{name: "meshery design", path: "designs/design.yml", wantKind: core.MesheryDesign, wantScore: ScoreMesheryDesign, wantInteresting: true},
		{name: "suffixed meshery design", path: "designs/istio.design.json", wantKind: core.MesheryDesign, wantScore: ScoreMesheryDesign, wantInteresting: true},
		{name: "chart archive tgz", path: "dist/redis-1.0.0.tgz", wantKind: core.HelmChart, wantScore: ScoreChartArchive, wantInteresting: true},
		{name: "chart archive tar.gz", path: "dist/redis-1.0.0.tar.gz", wantKind: core.HelmChart, wantScore: ScoreChartArchive, wantInteresting: true},
		{name: "generic yaml", path: "manifests/deployment.yaml", wantKind: "", wantScore: ScoreGenericYAML, wantInteresting: true},
		{name: "generic json", path: "manifests/service.json", wantKind: "", wantScore: ScoreGenericJSON, wantInteresting: true},
		{name: "zip archive is not a chart", path: "frontend/assets.zip", wantKind: "", wantScore: 0, wantInteresting: false},
		{name: "gz dump is not a chart", path: "logs/dump.gz", wantKind: "", wantScore: 0, wantInteresting: false},
		{name: "tar archive is not a chart", path: "vendor/deps.tar", wantKind: "", wantScore: 0, wantInteresting: false},
		{name: "uninteresting", path: "README.md", wantKind: "", wantScore: 0, wantInteresting: false},
		{name: "source file", path: "main.go", wantKind: "", wantScore: 0, wantInteresting: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, score, interesting := classifyPath(tt.path)
			if kind != tt.wantKind {
				t.Errorf("expected kind %q, got %q", tt.wantKind, kind)
			}
			if score != tt.wantScore {
				t.Errorf("expected score %d, got %d", tt.wantScore, score)
			}
			if interesting != tt.wantInteresting {
				t.Errorf("expected interesting to be %t, got %t", tt.wantInteresting, interesting)
			}
		})
	}
}

func TestRankTreeOrdersAndFilters(t *testing.T) {
	tests := []struct {
		name      string
		root      string
		maxSize   int64
		entries   []githubTreeEntry
		wantPaths []string
	}{
		{
			name:    "ranks interesting files and drops the rest",
			root:    "/**",
			maxSize: 1000,
			entries: []githubTreeEntry{
				{Type: "blob", Path: "README.md", SHA: "a", Size: 10},
				{Type: "blob", Path: "manifests/deployment.yaml", SHA: "b", Size: 10},
				{Type: "blob", Path: "designs/design.yml", SHA: "c", Size: 10},
				{Type: "blob", Path: "docker-compose.yml", SHA: "d", Size: 10},
				{Type: "blob", Path: "overlays/kustomization.yaml", SHA: "e", Size: 10},
				{Type: "blob", Path: "charts/redis/Chart.yaml", SHA: "f", Size: 10},
				{Type: "blob", Path: "dist/redis.tgz", SHA: "g", Size: 10},
				{Type: "blob", Path: "svc.json", SHA: "h", Size: 10},
				{Type: "tree", Path: "charts", SHA: "i"},
			},
			wantPaths: []string{
				"charts/redis/Chart.yaml",
				"overlays/kustomization.yaml",
				"docker-compose.yml",
				"designs/design.yml",
				"dist/redis.tgz",
				"manifests/deployment.yaml",
				"svc.json",
			},
		},
		{
			name:    "shallower paths win ties",
			root:    "/**",
			maxSize: 1000,
			entries: []githubTreeEntry{
				{Type: "blob", Path: "a/b/c/Chart.yaml", SHA: "a", Size: 10},
				{Type: "blob", Path: "Chart.yaml", SHA: "b", Size: 10},
				{Type: "blob", Path: "a/Chart.yaml", SHA: "c", Size: 10},
			},
			wantPaths: []string{"Chart.yaml", "a/Chart.yaml", "a/b/c/Chart.yaml"},
		},
		{
			name:    "oversized blobs are dropped before download",
			maxSize: 100,
			entries: []githubTreeEntry{
				{Type: "blob", Path: "small.yaml", SHA: "a", Size: 99},
				{Type: "blob", Path: "huge.yaml", SHA: "b", Size: 101},
			},
			wantPaths: []string{"small.yaml"},
		},
		{
			name:    "recursive root scopes to the subtree",
			root:    "charts/**",
			maxSize: 1000,
			entries: []githubTreeEntry{
				{Type: "blob", Path: "charts/redis/Chart.yaml", SHA: "a", Size: 10},
				{Type: "blob", Path: "charts/values.yaml", SHA: "b", Size: 10},
				{Type: "blob", Path: "designs/design.yml", SHA: "c", Size: 10},
			},
			wantPaths: []string{"charts/redis/Chart.yaml", "charts/values.yaml"},
		},
		{
			name:    "symlinks are never ranked",
			root:    "/**",
			maxSize: 1000,
			entries: []githubTreeEntry{
				{Type: "blob", Mode: "120000", Path: "docker-compose.yml", SHA: "a", Size: 25},
				{Type: "blob", Mode: "100644", Path: "deploy/docker-compose.yml", SHA: "b", Size: 40},
			},
			wantPaths: []string{"deploy/docker-compose.yml"},
		},
		{
			name:    "a root naming one exact file delivers it whatever it is called",
			root:    "scripts/install.sh",
			maxSize: 1000,
			entries: []githubTreeEntry{
				{Type: "blob", Path: "scripts/install.sh", SHA: "a", Size: 10},
				{Type: "blob", Path: "scripts/values.yaml", SHA: "b", Size: 10},
			},
			wantPaths: []string{"scripts/install.sh"},
		},
		{
			name:    "a walk's non-recursive root keeps only direct children",
			root:    "charts",
			maxSize: 1000,
			entries: []githubTreeEntry{
				{Type: "blob", Path: "charts/redis/Chart.yaml", SHA: "a", Size: 10},
				{Type: "blob", Path: "charts/values.yaml", SHA: "b", Size: 10},
			},
			wantPaths: []string{"charts/values.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGit().MaxFileSize(tt.maxSize)
			if tt.root != "" {
				g = g.Root(tt.root)
			}

			got := []string{}
			for _, candidate := range g.rankTree(tt.entries, g.recurse) {
				got = append(got, candidate.Path)
			}

			if !reflect.DeepEqual(got, tt.wantPaths) {
				t.Errorf("expected candidates %v, got %v", tt.wantPaths, got)
			}
		})
	}
}

func TestListInterestingFilesFetchesNoBlobs(t *testing.T) {
	stub := &githubAPIStub{
		commitSHA: "commit-sha",
		tree: githubTreeAPI{
			SHA: "tree-sha",
			Tree: []githubTreeEntry{
				{Type: "blob", Path: "Chart.yaml", SHA: "chart-blob", Size: 12},
				{Type: "blob", Path: "README.md", SHA: "readme-blob", Size: 12},
			},
		},
		blobs: map[string]string{"chart-blob": "name: redis"},
	}
	server := stub.server(t)

	listing, err := apiGit(server).
		Branch("release").
		Token("s3cret").
		ListInterestingFiles(context.Background())
	if err != nil {
		t.Fatalf("ListInterestingFiles() returned error: %v", err)
	}

	if listing.CommitSHA != "commit-sha" {
		t.Errorf("expected commit SHA %q, got %q", "commit-sha", listing.CommitSHA)
	}
	if listing.Truncated {
		t.Error("expected the listing not to be truncated")
	}
	if len(listing.Candidates) != 1 || listing.Candidates[0].Path != "Chart.yaml" {
		t.Fatalf("expected only Chart.yaml to be a candidate, got %+v", listing.Candidates)
	}
	if listing.Candidates[0].SHA != "chart-blob" || listing.Candidates[0].Size != 12 {
		t.Errorf("expected candidate metadata to come from the tree, got %+v", listing.Candidates[0])
	}
	if listing.Candidates[0].Kind != core.HelmChart {
		t.Errorf("expected inferred kind %q, got %q", core.HelmChart, listing.Candidates[0].Kind)
	}

	auth, refs, blobs := stub.snapshot()
	if len(blobs) != 0 {
		t.Errorf("expected no blob to be fetched while listing, got %v", blobs)
	}
	if !reflect.DeepEqual(refs, []string{"release"}) {
		t.Errorf("expected the branch to be resolved, got %v", refs)
	}
	for _, header := range auth {
		if header != "Bearer s3cret" {
			t.Errorf("expected every request to carry the bearer token, got %q", header)
		}
	}
}

func TestWalkContextUsesTreesPathAndFetchesSelectedBlobs(t *testing.T) {
	stub := &githubAPIStub{
		commitSHA: "commit-sha",
		tree: githubTreeAPI{
			Tree: []githubTreeEntry{
				{Type: "blob", Path: "Chart.yaml", SHA: "chart-blob", Size: 11},
				{Type: "blob", Path: "README.md", SHA: "readme-blob", Size: 11},
			},
		},
		blobs: map[string]string{"chart-blob": "name: redis"},
	}
	server := stub.server(t)

	intercepted := map[string]string{}
	stages := []ProgressStage{}
	err := apiGit(server).
		UseGithubAPI().
		Timeout(30 * time.Second).
		RegisterProgressHook(func(update ProgressUpdate) {
			stages = append(stages, update.Stage)
		}).
		RegisterFileInterceptor(func(file File) error {
			intercepted[file.Path] = file.Content
			return nil
		}).
		WalkContext(context.Background())
	if err != nil {
		t.Fatalf("WalkContext() returned error: %v", err)
	}

	if len(intercepted) != 1 || intercepted["Chart.yaml"] != "name: redis" {
		t.Fatalf("expected only Chart.yaml to be intercepted with its decoded contents, got %v", intercepted)
	}

	_, _, blobs := stub.snapshot()
	if !reflect.DeepEqual(blobs, []string{"chart-blob"}) {
		t.Errorf("expected only the surviving blob to be downloaded, got %v", blobs)
	}

	for _, want := range []ProgressStage{ProgressStageResolveRef, ProgressStageListTree, ProgressStageRank, ProgressStageFetchBlob} {
		if !containsStage(stages, want) {
			t.Errorf("expected progress stage %q to be reported, got %v", want, stages)
		}
	}
	if containsStage(stages, ProgressStageClone) {
		t.Error("did not expect the walk to fall back to a clone")
	}
}

func TestTreeWalkFallsBackToClone(t *testing.T) {
	tests := []struct {
		name        string
		truncated   bool
		dirIntercep bool
		wantWalked  bool
	}{
		{name: "truncated tree falls back", truncated: true, wantWalked: false},
		{name: "complete tree is walked over the api", truncated: false, wantWalked: true},
		{name: "directory interception falls back", dirIntercep: true, wantWalked: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &githubAPIStub{
				commitSHA: "commit-sha",
				tree: githubTreeAPI{
					Truncated: tt.truncated,
					Tree:      []githubTreeEntry{{Type: "blob", Path: "Chart.yaml", SHA: "chart-blob", Size: 11}},
				},
				blobs: map[string]string{"chart-blob": "name: redis"},
			}
			server := stub.server(t)

			g := apiGit(server).
				RegisterFileInterceptor(func(File) error { return nil })
			if tt.dirIntercep {
				g = g.RegisterDirInterceptor(func(Directory) error { return nil })
			}

			walked, err := g.treeWalk(context.Background())
			if err != nil {
				t.Fatalf("treeWalk() returned error: %v", err)
			}
			if walked != tt.wantWalked {
				t.Errorf("expected treeWalk to report walked=%t, got %t", tt.wantWalked, walked)
			}

			_, _, blobs := stub.snapshot()
			if !tt.wantWalked && len(blobs) != 0 {
				t.Errorf("expected no blob to be downloaded before falling back, got %v", blobs)
			}
		})
	}
}

func TestIsGithubHost(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    bool
		wantErr bool
	}{
		{name: "default base url", baseURL: "https://github.com", want: true},
		{name: "uppercase host", baseURL: "https://GitHub.com", want: true},
		{name: "github enterprise", baseURL: "https://github.example.com", want: false},
		{name: "gitlab", baseURL: "https://gitlab.com", want: false},
		{name: "local file url", baseURL: "file:///tmp/repos", want: false},
		{name: "unparseable", baseURL: "https://github.com:not-a-port", want: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGit().BaseURL(tt.baseURL).isGithubHost()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error for an unparseable base URL")
				}
				if code := meshkiterrors.GetCode(err); code != ErrInvalidBaseURLCode {
					t.Fatalf("expected error code %q, got %q", ErrInvalidBaseURLCode, code)
				}
				return
			}
			if err != nil {
				t.Fatalf("isGithubHost() returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected isGithubHost to be %t for %q, got %t", tt.want, tt.baseURL, got)
			}
		})
	}
}

func TestWalkContextKeepsGoGitForNonGithubHosts(t *testing.T) {
	// A non-github.com host must never reach the GitHub API, even with the
	// hybrid crawl enabled; the walk is expected to fail at the clone instead.
	stub := &githubAPIStub{commitSHA: "commit-sha"}
	server := stub.server(t)

	err := apiGit(server).
		BaseURL("https://git.example.com").
		UseGithubAPI().
		Timeout(5 * time.Second).
		RegisterFileInterceptor(func(File) error { return nil }).
		WalkContext(context.Background())
	if err == nil {
		t.Fatal("expected the clone against a non-existent host to fail")
	}
	if code := meshkiterrors.GetCode(err); code != ErrCloningRepoCode {
		t.Fatalf("expected the walk to fail while cloning, got error code %q: %v", code, err)
	}

	auth, _, _ := stub.snapshot()
	if len(auth) != 0 {
		t.Errorf("expected no GitHub API request for a non-github.com host, got %d", len(auth))
	}
}

func TestResolveRefSurfacesAPIErrorsWithoutTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := fmt.Fprint(w, `{"message":"Not Found"}`); err != nil {
			t.Errorf("failed to write stub response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	_, err := apiGit(server).
		Branch("main").
		Token("s3cret").
		ListInterestingFiles(context.Background())
	if err == nil {
		t.Fatal("expected ListInterestingFiles to fail when the ref cannot be resolved")
	}
	if code := meshkiterrors.GetCode(err); code != ErrResolvingGitRefCode {
		t.Fatalf("expected error code %q, got %q", ErrResolvingGitRefCode, code)
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatal("the access token must never appear in an error message")
	}
}

func TestFetchCandidatesReportsBlobFailures(t *testing.T) {
	stub := &githubAPIStub{commitSHA: "commit-sha", blobs: map[string]string{}}
	server := stub.server(t)

	err := apiGit(server).
		RegisterFileInterceptor(func(File) error { return nil }).
		FetchCandidates(context.Background(), []CandidateFile{{Path: "Chart.yaml", Name: "Chart.yaml", SHA: "missing"}})
	if err == nil {
		t.Fatal("expected FetchCandidates to fail for a missing blob")
	}
	if code := meshkiterrors.GetCode(err); code != ErrFetchingGitBlobCode {
		t.Fatalf("expected error code %q, got %q", ErrFetchingGitBlobCode, code)
	}
}

func TestListInterestingFilesResolvesTheConfiguredReference(t *testing.T) {
	tests := []struct {
		name            string
		branch          string
		referenceName   string
		wantRef         string
		wantRepoLookups int
	}{
		{
			name:    "neither set resolves the default branch through HEAD",
			wantRef: "HEAD",
		},
		{
			name:    "an explicit branch is used as is",
			branch:  "release",
			wantRef: "release",
		},
		{
			name:          "an explicit reference name wins over a branch",
			branch:        "release",
			referenceName: "refs/tags/v1.2.3",
			wantRef:       "v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &githubAPIStub{
				commitSHA: "commit-sha",
				tree:      githubTreeAPI{Tree: []githubTreeEntry{{Type: "blob", Path: "Chart.yaml", SHA: "chart-blob", Size: 11}}},
			}
			server := stub.server(t)

			g := apiGit(server)
			if tt.branch != "" {
				g = g.Branch(tt.branch)
			}
			if tt.referenceName != "" {
				g = g.ReferenceName(tt.referenceName)
			}

			if _, err := g.ListInterestingFiles(context.Background()); err != nil {
				t.Fatalf("ListInterestingFiles() returned error: %v", err)
			}

			_, refs, _ := stub.snapshot()
			if !reflect.DeepEqual(refs, []string{tt.wantRef}) {
				t.Errorf("expected the tree to be listed from %q, got %v", tt.wantRef, refs)
			}
			if got := stub.repoLookupCount(); got != tt.wantRepoLookups {
				t.Errorf("expected %d repository lookups, got %d", tt.wantRepoLookups, got)
			}
		})
	}
}

func TestFetchCandidatesDownloadsConcurrentlyAndDeliversInOrder(t *testing.T) {
	const candidateCount = blobFetchConcurrency * 3

	inFlight := make(chan string, candidateCount)
	release := make(chan struct{})
	releaseAll := sync.OnceFunc(func() { close(release) })
	stub := &githubAPIStub{
		blobs: map[string]string{},
		onBlob: func(sha string) {
			inFlight <- sha
			<-release
		},
	}
	candidates := make([]CandidateFile, 0, candidateCount)
	for i := 0; i < candidateCount; i++ {
		sha := fmt.Sprintf("blob-%02d", i)
		stub.blobs[sha] = fmt.Sprintf("content-%02d", i)
		candidates = append(candidates, CandidateFile{Path: fmt.Sprintf("%02d.yaml", i), Name: fmt.Sprintf("%02d.yaml", i), SHA: sha})
	}
	server := stub.server(t)
	// Registered after the server's own cleanup, which runs first and would
	// otherwise wait forever on a handler a failed assertion left parked.
	t.Cleanup(releaseAll)

	delivered := []string{}
	done := make(chan error, 1)
	go func() {
		done <- apiGit(server).
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, file.Path)
				return nil
			}).
			FetchCandidates(context.Background(), candidates)
	}()

	// Every handler stays parked until release is closed, so a serial fetcher
	// can never put a second request in flight: waiting for two is what rules
	// it out, whatever blobFetchConcurrency is set to.
	const minimumOverlap = 2
	for i := 0; i < blobFetchConcurrency; i++ {
		select {
		case <-inFlight:
		case <-time.After(10 * time.Second):
			if i < minimumOverlap {
				t.Fatalf("expected blob downloads to overlap, only %d reached the server", i)
			}
			t.Fatalf("expected %d blob downloads to overlap, only %d reached the server", blobFetchConcurrency, i)
		}
	}

	// ... and no more than that, so a large selection cannot open an unbounded
	// number of connections.
	select {
	case sha := <-inFlight:
		t.Fatalf("expected at most %d downloads in flight, %q made it %d", blobFetchConcurrency, sha, blobFetchConcurrency+1)
	case <-time.After(100 * time.Millisecond):
	}

	releaseAll()
	if err := <-done; err != nil {
		t.Fatalf("FetchCandidates() returned error: %v", err)
	}

	wantOrder := make([]string, 0, candidateCount)
	for _, candidate := range candidates {
		wantOrder = append(wantOrder, candidate.Path)
	}
	if !reflect.DeepEqual(delivered, wantOrder) {
		t.Errorf("expected every candidate to be delivered in order, got %v", delivered)
	}

	_, _, blobs := stub.snapshot()
	if len(blobs) != candidateCount {
		t.Errorf("expected all %d blobs to be downloaded, got %d", candidateCount, len(blobs))
	}
}

func TestTreeWalkFetchesEveryRankedCandidateHoweverLargeTheListing(t *testing.T) {
	// One route, one result set: a large listing is fetched in full rather
	// than swapped for a clone that would deliver a different set of files.
	const candidateCount = 600

	stub := &githubAPIStub{commitSHA: "commit-sha", blobs: map[string]string{}}
	wantPaths := make([]string, 0, candidateCount)
	for i := 0; i < candidateCount; i++ {
		sha := fmt.Sprintf("blob-%04d", i)
		candidatePath := fmt.Sprintf("manifests/%04d.yaml", i)
		stub.blobs[sha] = "kind: ConfigMap"
		stub.tree.Tree = append(stub.tree.Tree, githubTreeEntry{Type: "blob", Path: candidatePath, SHA: sha, Size: 14})
		wantPaths = append(wantPaths, candidatePath)
	}
	stub.tree.Tree = append(stub.tree.Tree, githubTreeEntry{Type: "blob", Path: "manifests/README.md", SHA: "readme-blob", Size: 14})
	server := stub.server(t)

	delivered := []string{}
	walked, err := apiGit(server).
		Root("manifests/**").
		RegisterFileInterceptor(func(file File) error {
			delivered = append(delivered, file.Path)
			return nil
		}).
		treeWalk(context.Background())
	if err != nil {
		t.Fatalf("treeWalk() returned error: %v", err)
	}
	if !walked {
		t.Fatal("expected the API route to fetch the whole listing rather than fall back to a clone")
	}
	if !reflect.DeepEqual(delivered, wantPaths) {
		t.Fatalf("expected all %d ranked candidates to be delivered, got %d", len(wantPaths), len(delivered))
	}

	_, _, blobs := stub.snapshot()
	if len(blobs) != candidateCount {
		t.Errorf("expected %d blobs to be downloaded, got %d", candidateCount, len(blobs))
	}
}

func TestListingAPIRefusesRepositoriesOnOtherHosts(t *testing.T) {
	// The API endpoint is github.com's whatever BaseURL says, so a walker
	// configured for another forge must be refused before its token travels.
	stub := &githubAPIStub{
		commitSHA: "commit-sha",
		tree:      githubTreeAPI{Tree: []githubTreeEntry{{Type: "blob", Path: "Chart.yaml", SHA: "chart-blob", Size: 11}}},
		blobs:     map[string]string{"chart-blob": "name: redis"},
	}
	server := stub.server(t)

	elsewhere := func() *Git {
		return apiGit(server).
			BaseURL("https://gitlab.com").
			Token("s3cret").
			RegisterFileInterceptor(func(File) error { return nil })
	}

	_, err := elsewhere().ListInterestingFiles(context.Background())
	if err == nil {
		t.Fatal("expected ListInterestingFiles to refuse a repository that is not on github.com")
	}
	if code := meshkiterrors.GetCode(err); code != ErrInvalidBaseURLCode {
		t.Fatalf("expected error code %q, got %q: %v", ErrInvalidBaseURLCode, code, err)
	}

	err = elsewhere().FetchCandidates(context.Background(), []CandidateFile{{Path: "Chart.yaml", Name: "Chart.yaml", SHA: "chart-blob"}})
	if err == nil {
		t.Fatal("expected FetchCandidates to refuse a repository that is not on github.com")
	}
	if code := meshkiterrors.GetCode(err); code != ErrInvalidBaseURLCode {
		t.Fatalf("expected error code %q, got %q: %v", ErrInvalidBaseURLCode, code, err)
	}

	auth, _, blobs := stub.snapshot()
	if len(auth) != 0 {
		t.Errorf("expected the access token never to reach the GitHub API, got %d request(s) carrying %v", len(auth), auth)
	}
	if len(blobs) != 0 {
		t.Errorf("expected no blob to be downloaded from another host's repository, got %v", blobs)
	}
}

func TestResolveRefSendsASlashedBranchAsSeveralPathSegments(t *testing.T) {
	// A branch like release/1.2 is spelled as a path by the commits endpoint,
	// so the separator has to survive onto the wire rather than travel as one
	// escaped segment.
	stub := &githubAPIStub{
		commitSHA: "commit-sha",
		tree:      githubTreeAPI{Tree: []githubTreeEntry{{Type: "blob", Mode: "100644", Path: "Chart.yaml", SHA: "chart-blob", Size: 11}}},
	}
	server := stub.server(t)

	listing, err := apiGit(server).Branch("release/1.2").ListInterestingFiles(context.Background())
	if err != nil {
		t.Fatalf("ListInterestingFiles() returned error: %v", err)
	}
	if len(listing.Candidates) != 1 {
		t.Fatalf("expected the tree to be listed once the branch resolved, got %+v", listing.Candidates)
	}

	uris := stub.requestedCommitURIs()
	if len(uris) != 1 {
		t.Fatalf("expected the commits endpoint to be requested once, got %v", uris)
	}
	if want := "/repos/owner/repo/commits/release/1.2"; uris[0] != want {
		t.Errorf("expected the branch to arrive as %q, got %q", want, uris[0])
	}
}

func TestResolveRefRefusesReferencesGitWouldRefuse(t *testing.T) {
	// A segment git will not accept is not a name the commits endpoint can be
	// asked for, and "." or ".." would travel as a path element rather than as
	// part of the reference, so the request is never made.
	stub := &githubAPIStub{
		commitSHA: "commit-sha",
		tree:      githubTreeAPI{Tree: []githubTreeEntry{{Type: "blob", Mode: "100644", Path: "Chart.yaml", SHA: "chart-blob", Size: 11}}},
	}
	server := stub.server(t)

	refs := []string{
		"../other/commits/main",
		"release/../../other/commits/main",
		"release/.",
		"feature/.hidden",
		"release/1.2.lock",
		"release//1.2",
	}

	for _, ref := range refs {
		t.Run(ref, func(t *testing.T) {
			_, err := apiGit(server).Branch(ref).ListInterestingFiles(context.Background())
			if err == nil {
				t.Fatalf("expected %q to be refused", ref)
			}
			if code := meshkiterrors.GetCode(err); code != ErrResolvingGitRefCode {
				t.Fatalf("expected error code %q, got %q: %v", ErrResolvingGitRefCode, code, err)
			}
		})
	}

	if received := stub.receivedRequests(); len(received) != 0 {
		t.Errorf("expected a refused reference to be requested from nowhere, got %v", received)
	}
}

func TestFetchCandidatesHonoursTheFileSizeLimit(t *testing.T) {
	// The limit has to hold whatever the candidate claims, because a picker's
	// selection travels through a client before it comes back as candidates.
	const limit = 3000

	stub := &githubAPIStub{blobs: map[string]string{
		"at-limit":  strings.Repeat("y", limit),
		"oversized": strings.Repeat("y", limit*4),
	}}
	server := stub.server(t)

	fetch := func(t *testing.T, candidate CandidateFile) (string, error) {
		t.Helper()

		delivered := ""
		err := apiGit(server).
			MaxFileSize(limit).
			RegisterFileInterceptor(func(file File) error {
				delivered = file.Content
				return nil
			}).
			FetchCandidates(context.Background(), []CandidateFile{candidate})
		return delivered, err
	}

	t.Run("a blob at the limit is delivered", func(t *testing.T) {
		delivered, err := fetch(t, CandidateFile{Path: "at-limit.yaml", Name: "at-limit.yaml", SHA: "at-limit", Size: limit})
		if err != nil {
			t.Fatalf("FetchCandidates() returned error: %v", err)
		}
		if len(delivered) != limit {
			t.Errorf("expected the whole %d byte blob to be delivered, got %d bytes", limit, len(delivered))
		}
	})

	t.Run("a blob past the limit is refused even when the candidate understates it", func(t *testing.T) {
		delivered, err := fetch(t, CandidateFile{Path: "oversized.yaml", Name: "oversized.yaml", SHA: "oversized"})
		if err == nil {
			t.Fatal("expected a blob past the limit to be refused")
		}
		if code := meshkiterrors.GetCode(err); code != ErrInvalidSizeFileCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrInvalidSizeFileCode, code, err)
		}
		if delivered != "" {
			t.Errorf("expected nothing to be delivered, got %d bytes", len(delivered))
		}
	})

	t.Run("a candidate reporting an oversize is refused without a request", func(t *testing.T) {
		stub := &githubAPIStub{blobs: map[string]string{"oversized": strings.Repeat("y", limit*4)}}
		server := stub.server(t)

		err := apiGit(server).
			MaxFileSize(limit).
			RegisterFileInterceptor(func(File) error { return nil }).
			FetchCandidates(context.Background(), []CandidateFile{{Path: "oversized.yaml", Name: "oversized.yaml", SHA: "oversized", Size: limit + 1}})
		if err == nil {
			t.Fatal("expected a candidate reporting an oversize to be refused")
		}
		if code := meshkiterrors.GetCode(err); code != ErrInvalidSizeFileCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrInvalidSizeFileCode, code, err)
		}

		_, _, blobs := stub.snapshot()
		if len(blobs) != 0 {
			t.Errorf("expected no blob to be requested, got %v", blobs)
		}
	})
}

func TestListInterestingFilesScopesToRoot(t *testing.T) {
	tree := githubTreeAPI{Tree: []githubTreeEntry{
		{Type: "blob", Path: "README.md", SHA: "readme-blob", Size: 10},
		{Type: "blob", Path: "top.yaml", SHA: "top-blob", Size: 10},
		{Type: "blob", Path: "charts/nginx/Chart.yaml", SHA: "nginx-blob", Size: 10},
		{Type: "blob", Path: "charts/redis/Chart.yaml", SHA: "redis-blob", Size: 10},
	}}

	listPaths := func(t *testing.T, scope func(*Git) *Git) []string {
		t.Helper()

		stub := &githubAPIStub{commitSHA: "commit-sha", tree: tree}
		listing, err := scope(apiGit(stub.server(t))).ListInterestingFiles(context.Background())
		if err != nil {
			t.Fatalf("ListInterestingFiles() returned error: %v", err)
		}

		paths := []string{}
		for _, candidate := range listing.Candidates {
			paths = append(paths, candidate.Path)
		}
		return paths
	}

	wholeRepository := []string{"charts/nginx/Chart.yaml", "charts/redis/Chart.yaml", "top.yaml"}

	// A picker that sends no subdirectory reaches the walker either as a Root
	// that was never called or as Root("")/Root("/"), and all three mean the
	// same thing to the listing.
	unscoped := []struct {
		name  string
		scope func(*Git) *Git
	}{
		{name: "an unset root lists the whole repository", scope: func(g *Git) *Git { return g }},
		{name: "an empty root lists the whole repository", scope: func(g *Git) *Git { return g.Root("") }},
		{name: "a slash root lists the whole repository", scope: func(g *Git) *Git { return g.Root("/") }},
	}

	for _, tt := range unscoped {
		t.Run(tt.name, func(t *testing.T) {
			if got := listPaths(t, tt.scope); !reflect.DeepEqual(got, wholeRepository) {
				t.Errorf("expected the nested layout to be listed as %v, got %v", wholeRepository, got)
			}
		})
	}

	t.Run("a root still narrows the listing", func(t *testing.T) {
		want := []string{"charts/redis/Chart.yaml"}
		if got := listPaths(t, func(g *Git) *Git { return g.Root("charts/redis") }); !reflect.DeepEqual(got, want) {
			t.Errorf("expected the listing to be scoped to the root, got %v", got)
		}
	})

	t.Run("a root narrows which subtree is listed, not how deep", func(t *testing.T) {
		// Every chart in the picked folder is nested one level down, which is
		// the layout the ranking exists for.
		want := []string{"charts/nginx/Chart.yaml", "charts/redis/Chart.yaml"}
		if got := listPaths(t, func(g *Git) *Git { return g.Root("charts") }); !reflect.DeepEqual(got, want) {
			t.Errorf("expected the whole subtree to be listed as %v, got %v", want, got)
		}
	})

	t.Run("a walk keeps its own non-recursive root", func(t *testing.T) {
		stub := &githubAPIStub{commitSHA: "commit-sha", tree: tree, blobs: map[string]string{}}

		delivered := []string{}
		walked, err := apiGit(stub.server(t)).
			Root("charts").
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, file.Path)
				return nil
			}).
			treeWalk(context.Background())
		if err != nil {
			t.Fatalf("treeWalk() returned error: %v", err)
		}
		if !walked {
			t.Fatal("expected the API route to carry out the walk")
		}
		if len(delivered) != 0 {
			t.Errorf("expected a walk to stay in the named directory, got %v", delivered)
		}
	})

	// Walk's historical Root semantics are back-compat and unaffected by what
	// an unscoped Root means to the listing API.
	walkScopes := []struct {
		name  string
		scope func(*Git) *Git
	}{
		{name: "an unset root leaves a walk on its top level", scope: func(g *Git) *Git { return g }},
		{name: "an empty root leaves a walk on its top level", scope: func(g *Git) *Git { return g.Root("") }},
	}

	for _, tt := range walkScopes {
		t.Run(tt.name, func(t *testing.T) {
			stub := &githubAPIStub{commitSHA: "commit-sha", tree: tree, blobs: map[string]string{"top-blob": "kind: ConfigMap"}}

			delivered := []string{}
			walked, err := tt.scope(apiGit(stub.server(t))).
				RegisterFileInterceptor(func(file File) error {
					delivered = append(delivered, file.Path)
					return nil
				}).
				treeWalk(context.Background())
			if err != nil {
				t.Fatalf("treeWalk() returned error: %v", err)
			}
			if !walked {
				t.Fatal("expected the API route to carry out the walk")
			}
			if want := []string{"top.yaml"}; !reflect.DeepEqual(delivered, want) {
				t.Errorf("expected the walk to stay on the top level as %v, got %v", want, delivered)
			}
		})
	}
}

func TestListInterestingFilesRejectsAZeroMaxFileSize(t *testing.T) {
	// A zero limit drops every file, so the listing has to report the
	// misconfiguration the way a walk does rather than look like an empty
	// repository.
	stub := &githubAPIStub{
		commitSHA: "commit-sha",
		tree:      githubTreeAPI{Tree: []githubTreeEntry{{Type: "blob", Path: "Chart.yaml", SHA: "chart-blob", Size: 11}}},
	}
	server := stub.server(t)

	_, err := apiGit(server).MaxFileSize(0).ListInterestingFiles(context.Background())
	if err == nil {
		t.Fatal("expected ListInterestingFiles to reject a zero max file size")
	}
	if code := meshkiterrors.GetCode(err); code != ErrInvalidSizeFileCode {
		t.Fatalf("expected error code %q, got %q: %v", ErrInvalidSizeFileCode, code, err)
	}

	auth, _, _ := stub.snapshot()
	if len(auth) != 0 {
		t.Errorf("expected no request to be made for an unusable size limit, got %d", len(auth))
	}
}

func containsStage(stages []ProgressStage, want ProgressStage) bool {
	for _, stage := range stages {
		if stage == want {
			return true
		}
	}
	return false
}
