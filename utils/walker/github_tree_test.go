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

	mu            sync.Mutex
	authorization []string
	requestedRefs []string
	fetchedBlobs  []string
}

func (s *githubAPIStub) server(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/commits/", func(w http.ResponseWriter, r *http.Request) {
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

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
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
		{name: "docker compose", path: "docker-compose.yml", wantKind: core.DockerCompose, wantScore: ScoreDockerCompose, wantInteresting: true},
		{name: "compose", path: "deploy/compose.yaml", wantKind: core.DockerCompose, wantScore: ScoreDockerCompose, wantInteresting: true},
		{name: "meshery design", path: "designs/design.yml", wantKind: core.MesheryDesign, wantScore: ScoreMesheryDesign, wantInteresting: true},
		{name: "suffixed meshery design", path: "designs/istio.design.json", wantKind: core.MesheryDesign, wantScore: ScoreMesheryDesign, wantInteresting: true},
		{name: "chart archive tgz", path: "dist/redis-1.0.0.tgz", wantKind: core.HelmChart, wantScore: ScoreChartArchive, wantInteresting: true},
		{name: "chart archive tar.gz", path: "dist/redis-1.0.0.tar.gz", wantKind: core.HelmChart, wantScore: ScoreChartArchive, wantInteresting: true},
		{name: "generic yaml", path: "manifests/deployment.yaml", wantKind: "", wantScore: ScoreGenericYAML, wantInteresting: true},
		{name: "generic json", path: "manifests/service.json", wantKind: "", wantScore: ScoreGenericJSON, wantInteresting: true},
		{name: "uninteresting", path: "README.md", wantKind: "", wantScore: 0, wantInteresting: false},
		{name: "source file", path: "main.go", wantKind: "", wantScore: 0, wantInteresting: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, score, interesting := ClassifyPath(tt.path)
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
			name:    "non-recursive root keeps only direct children",
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
			for _, candidate := range g.rankTree(tt.entries) {
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

	listing, err := NewGit().
		Owner("owner").
		Repo("repo").
		Branch("release").
		APIBaseURL(server.URL).
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
	err := NewGit().
		Owner("owner").
		Repo("repo").
		APIBaseURL(server.URL).
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

			g := NewGit().
				Owner("owner").
				Repo("repo").
				APIBaseURL(server.URL).
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
		{name: "www host", baseURL: "https://www.github.com", want: true},
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

	err := NewGit().
		BaseURL("https://git.example.com").
		Owner("owner").
		Repo("repo").
		APIBaseURL(server.URL).
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

	_, err := NewGit().
		Owner("owner").
		Repo("repo").
		APIBaseURL(server.URL).
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

	err := NewGit().
		Owner("owner").
		Repo("repo").
		APIBaseURL(server.URL).
		RegisterFileInterceptor(func(File) error { return nil }).
		FetchCandidates(context.Background(), []CandidateFile{{Path: "Chart.yaml", Name: "Chart.yaml", SHA: "missing"}})
	if err == nil {
		t.Fatal("expected FetchCandidates to fail for a missing blob")
	}
	if code := meshkiterrors.GetCode(err); code != ErrFetchingGitBlobCode {
		t.Fatalf("expected error code %q, got %q", ErrFetchingGitBlobCode, code)
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
