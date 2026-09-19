# Git walkers (`utils/walker`)

`utils/walker` is how MeshKit reads a remote repository during design and model import.
Meshery Server, Meshery Cloud, the Meshery extensions, and this repo's own
`generators/github` all drive it, so every exported symbol here is part of MeshKit's
published API and is only ever added to, never changed.

The package offers three walkers:

| Walker | Entry points | Transport |
|--------|--------------|-----------|
| `Git` | `Walk`, `WalkContext`, `ListInterestingFiles`, `FetchCandidates` | go-git clone, or the GitHub Git Trees + Blobs API |
| `Github` | `Walk`, `WalkContext` | GitHub Contents API |
| local | `WalkLocalDirectory` | filesystem |

## The hybrid GitHub crawl

`Git.Walk` has always shallow-cloned the whole repository and then filtered it, which is
expensive on large repositories. `Git.UseGithubAPI()` opts into a hybrid crawl instead:

1. **Resolve** the configured reference to a commit SHA (`GET /repos/{owner}/{repo}/commits/{ref}`).
2. **List** that commit's tree once, recursively (`GET /repos/{owner}/{repo}/git/trees/{sha}?recursive=1`).
3. **Filter and rank** the returned entries from their metadata alone - path, type and size.
4. **Fetch** blobs (`GET /repos/{owner}/{repo}/git/blobs/{sha}`) only for entries that survived,
   and hand each to the registered file interceptor.

The size limit set by `MaxFileSize` is applied against the size the tree already reports, so
an oversized blob is never downloaded at all.

The crawl is **opt-in**. Callers that do not enable it keep cloning exactly as before.

### When the crawl falls back to go-git

`WalkContext` falls back to the clone walk whenever the API route cannot answer completely:

- **The host is not github.com.** The host is read from the configured `BaseURL`, never assumed,
  so GitHub Enterprise, GitLab, Bitbucket and `file://` URLs keep taking the clone path.
- **The tree came back truncated.** GitHub caps a single tree response; `truncated: true` means
  the listing is incomplete, and only a clone sees every file.
- **A directory interceptor is registered.** Directory interception hands the interceptor a
  working-tree path (`helm.ConvertToK8sManifest` needs one), which the API route never produces.

Sparse and partial clone for large non-GitHub repositories, and GitLab/Bitbucket adapters, are
deliberately out of scope - see [ux/canvas-first-github-onboarding.md](ux/canvas-first-github-onboarding.md).

## Ranked interesting files, before any download

`Git.ListInterestingFiles(ctx)` returns the ranked candidate listing without downloading a
single blob, so Cloud and the extensions can render an import picker and then pass the user's
selection back to `Git.FetchCandidates(ctx, selected)`.

Ranking is **path based**, because it runs before any content exists. `ClassifyPath` assigns:

| Score | Constant | Matches | Inferred kind |
|------:|----------|---------|---------------|
| 100 | `ScoreHelmChartDefinition` | `Chart.yaml`, `Chart.yml` | `core.HelmChart` |
| 90 | `ScoreKustomization` | `kustomization.*` | `core.K8sKustomize` |
| 80 | `ScoreDockerCompose` | `docker-compose.*`, `compose.*` | `core.DockerCompose` |
| 70 | `ScoreMesheryDesign` | `design.yml`/`.yaml`/`.json`, `*.design.*` | `core.MesheryDesign` |
| 60 | `ScoreChartArchive` | chart and OCI archives (`.tgz`, `.tar.gz`, `.tar`, `.zip`, `.gz`) | `core.HelmChart` |
| 20 | `ScoreGenericYAML` | any other `.yaml`/`.yml` | *(empty)* |
| 10 | `ScoreGenericJSON` | any other `.json` | *(empty)* |

Ties break on path depth, then alphabetically, so a chart at the repository root outranks one
buried in a test fixture.

An empty `Kind` means the path is worth fetching but is not distinctive enough to name a type.
Identification proper remains the caller's job: run `files.IdentifyFile` once the contents are
in hand.

### Where the extension tables live

`ClassifyPath` reuses the same extension tables `files` parses with, rather than restating the
literals. Those tables live in the leaf package **`files/iacext`**: `files` imports
`utils/walker`, so a table owned by `files` and read by the walker would close an import cycle.
`files.ValidHelmChartFileExtensions` and `files.ValidKustomizeFileExtensions` are retained as
aliases of the `iacext` tables, so existing callers are unaffected.

## Branch, reference, context, progress and auth

- **Branch vs reference.** `ReferenceName` wins when set. Otherwise an explicitly set `Branch`
  is expanded to `refs/heads/<branch>`. A caller that sets neither still clones the remote's
  default branch - the `NewGit()` default of `"master"` (and `NewGithub()`'s `"main"`) is only
  a fallback for the API ref, never something forced onto a clone.
- **Context.** `WalkContext`, `ListInterestingFiles` and `FetchCandidates` all take a context;
  `Walk()` delegates with `context.Background()`. `Timeout(d)` bounds a whole traversal.
- **Progress.** `RegisterProgressHook` receives `ProgressUpdate` values as the walk moves
  through `resolve-ref`, `list-tree`, `rank`, `fetch-blob` and `clone`.
- **Auth.** `Token(t)` threads a GitHub App or OAuth token onto the API calls as a bearer token
  and onto the clone as `x-access-token` basic auth, for private repositories and the
  authenticated rate limit. The token is never logged, never placed in an error message and
  never reported through a progress hook.
