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
   When neither `Branch` nor `ReferenceName` was set the reference is `HEAD`, which that same
   endpoint resolves to the head commit of the repository's default branch - the branch the clone
   route would have checked out - so no extra request is needed to name it.
2. **List** that commit's tree once, recursively (`GET /repos/{owner}/{repo}/git/trees/{sha}?recursive=1`).
3. **Filter and rank** the returned entries from their metadata alone - path, type, mode and
   size. Symlinks are dropped here: the Trees API reports one as a blob whose content is the
   link target path, not the target's contents.
4. **Fetch** blobs (`GET /repos/{owner}/{repo}/git/blobs/{sha}`) only for entries that survived,
   and hand each to the registered file interceptor. Up to 8 blobs download at a time, while the
   interceptor is still called once at a time and in ranked order.

The size limit set by `MaxFileSize` is applied against the size the tree already reports, so
an oversized blob is never downloaded at all.

The crawl is **opt-in**. Callers that do not enable it keep cloning exactly as before.

### When the crawl falls back to go-git

`WalkContext` falls back to the clone walk whenever the API route cannot answer completely:

- **The host is not github.com.** The host is read from the configured `BaseURL`, never assumed,
  so GitHub Enterprise, GitLab, Bitbucket and `file://` URLs keep taking the clone path.
- **The tree came back truncated.** GitHub caps a single tree response; `truncated: true` means
  the listing is incomplete, so a clone reads the repository instead.
- **A directory interceptor is registered.** Directory interception needs a directory that
  exists, which the API route never produces, so the walk clones.

**All three of those clones are filtered.** Opting into `UseGithubAPI()` is what decides this, not
the reason the clone happened, so a caller that enables the flag and then points a connection at
GitHub Enterprise is filtered exactly as a truncated-tree fallback is: the clone route runs the
same classifier, so `.md`, `.txt`, `.sh` and extensionless files never reach the interceptor, and
it skips a file over `MaxFileSize` instead of failing the walk. Symlinks are skipped outright, as
they are on the API route, which never offers one. What the caller gets is the same filtered,
size-bounded **set** - not the same order: the clone delivers in `filepath.WalkDir`/`os.ReadDir`
lexical order, never in score order, so an interceptor must not assume `Chart.yaml` arrives before
`values.yaml`. Opting in also changes the paths the interceptors are handed - see **Paths** below.

A caller that never opted in keeps all of that behaviour: every file under `Root`, in the order it
always arrived, with today's oversize error. Symlinks are followed for those callers too, with one
limit - a link is read only while its target resolves inside the repository copy the walk made. A
link whose target lands outside that copy, or that cannot be resolved at all, is passed over
silently; the walk continues and nothing else changes. Targets are resolved with `lstat`/`readlink`
alone, so a file that will not be read is never opened.

Nothing else switches routes, and the size of the repository in particular does not: the
auto-fetch path issues **one blob request per ranked candidate**, however many there are. It is
also all-or-nothing only in name: candidates are delivered to the interceptor as they arrive, so
a failure partway through (a 404 on a blob, a rate limit) returns an error *after* the earlier
candidates have already been handed over, and nothing in the error says how far it got. A caller
that wants to spend fewer requests, or to restart where it stopped, picks the files itself -
`ListInterestingFiles` costs two requests and downloads nothing, and `FetchCandidates` then
fetches only the selection it is handed.

Sparse and partial clone for large non-GitHub repositories, and GitLab/Bitbucket adapters, are
deliberately out of scope - see [ux/canvas-first-github-onboarding.md](ux/canvas-first-github-onboarding.md).

## Ranked interesting files, before any download

`Git.ListInterestingFiles(ctx)` returns the ranked candidate listing without downloading a
single blob, so Cloud and the extensions can render an import picker and then pass the user's
selection back to `Git.FetchCandidates(ctx, selected)`.

Both are GitHub-only: unlike `WalkContext` there is no clone to fall back to, so a `BaseURL` on
any other host is refused rather than answered from github.com, which is where the API endpoint
points regardless of `BaseURL`. Refusing it is what keeps the access token from travelling to a
host the caller never configured.

**The listing is always recursive.** `Root` narrows *which* subtree is listed, never how deep, so
`Root("charts")` returns `charts/redis/Chart.yaml` as well as `charts/values.yaml`. An unscoped
`Root` therefore lists the whole repository, because a picker is asking what the repository holds;
unscoped means all of: never calling `Root`, `Root("")` and `Root("/")` - a picker that sends no
subdirectory reaches the walker as any of the three.

This is deliberately *not* what `Root` means to `Walk`/`WalkContext`, which keep their historical
scope for back-compatibility: there an unscoped `Root` is the top level only, a named `Root` is
that directory's own files, and `"/**"` is what asks for the subtree below it. The listing API is
new surface with no such obligation, and a picker that shows a folder means everything in it.

`MaxFileSize(0)` is rejected here (`ErrInvalidSizeFile`) exactly as it is on a walk, rather than
answered with an empty listing.

Ranking is **path based**, because it runs before any content exists. The classifier assigns:

| Score | Constant | Matches | Inferred kind |
|------:|----------|---------|---------------|
| 100 | `ScoreHelmChartDefinition` | `Chart.yaml`, `Chart.yml` | `core.HelmChart` |
| 90 | `ScoreKustomization` | `kustomization.yaml`, `kustomization.yml` | `core.K8sKustomize` |
| 80 | `ScoreDockerCompose` | `docker-compose.yaml`/`.yml`, `compose.yaml`/`.yml` | `core.DockerCompose` |
| 70 | `ScoreMesheryDesign` | `design` or `*.design` with `.yaml`/`.yml`/`.json` | `core.MesheryDesign` |
| 60 | `ScoreChartArchive` | chart and OCI archives: `.tgz`, `.tar.gz` | `core.HelmChart` |
| 20 | `ScoreGenericYAML` | any other `.yaml`/`.yml` | *(empty)* |
| 10 | `ScoreGenericJSON` | any other `.json` | *(empty)* |

Ties break on path depth, then alphabetically, so a chart at the repository root outranks one
buried in a test fixture. A `Root` naming one exact file always yields that file, whatever it is
called, matching what the clone route does with an explicitly named file.

Extensions are matched exactly: only `.tgz` and `.tar.gz` rank as chart archives, and only
`.yaml`/`.yml` as kustomizations, because that is what those files are in a repository tree. The
wider `files.ValidHelmChartFileExtensions` and `files.ValidKustomizeFileExtensions` tables
describe what an *uploaded* file may arrive as; applying them here would label every `.zip`,
`.gz` and `.tar` in a repository a Helm chart or a kustomization.

The classifier itself is not exported - `CandidateFile.Kind` and `CandidateFile.Score` carry its
output for every candidate, and the `Score*` constants above are what a picker needs to group or
threshold by.

An empty `Kind` means the path is worth fetching but is not distinctive enough to name a type.
Identification proper remains the caller's job: run `files.IdentifyFile` once the contents are
in hand.

## Branch, reference, context, progress and auth

- **Branch vs reference.** `ReferenceName` wins when set. Otherwise an explicitly set `Branch`
  is expanded to `refs/heads/<branch>`. A caller that sets neither gets the remote's default
  branch on both routes: the clone lets go-git pick it, and the API route resolves `HEAD`, which
  the commits endpoint answers with that branch's head commit. The `NewGit()` default of
  `"master"` (and `NewGithub()`'s `"main"`) only applies once `Branch` has been called, so it is
  never forced onto either route.
- **Context.** `WalkContext`, `ListInterestingFiles` and `FetchCandidates` all take a context;
  `Walk()` delegates with `context.Background()`. `Timeout(d)` bounds a whole traversal.
- **Progress.** `RegisterProgressHook` receives `ProgressUpdate` values as the walk moves
  through `resolve-ref`, `list-tree`, `rank`, `fetch-blob` and `clone`.
- **Paths.** Under `UseGithubAPI()` **every** path handed to an interceptor is
  repository-relative - `File.Path` and `Directory.Path` alike, on the API route and on the clone
  it falls back to - so one import handles one kind of path. Those paths **must not be opened
  from the filesystem**: on the API route nothing is written to disk at all, and on the clone
  route the temporary clone is deleted when the walk returns. A caller that needs real on-disk
  files - passing a directory to `helm.ConvertToK8sManifest`, or reading `File.Path` back off
  disk, as `generators/github` does - should **not** opt in. Callers that never opted in keep the
  absolute path into the (temporary) clone they have always received, byte for byte.
- **Auth.** `Token(t)` threads a GitHub App or OAuth token onto the API calls as a bearer token
  and onto the clone as `x-access-token` basic auth, for private repositories and the
  authenticated rate limit. The token is never logged, never placed in an error message and
  never reported through a progress hook.
