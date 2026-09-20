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

1. **Resolve** the configured reference to a commit SHA (`GET /repos/{owner}/{repo}/commits/{ref}`),
   asked for with `Accept: application/vnd.github.sha` so the answer is the SHA alone rather than
   a commit with a patch per changed file. When neither `Branch` nor `ReferenceName` was set the
   reference is `HEAD`, which that same endpoint resolves to the head commit of the repository's
   default branch - the branch the clone route would have checked out - so no extra request is
   needed to name it.
2. **List** that commit's tree once, recursively (`GET /repos/{owner}/{repo}/git/trees/{sha}?recursive=1`).
3. **Filter and rank** the returned entries from their metadata alone - path, type, mode and
   size. Symlinks are dropped here: the Trees API reports one as a blob whose content is the
   link target path, not the target's contents.
4. **Fetch** blobs (`GET /repos/{owner}/{repo}/git/blobs/{sha}`) only for entries that survived,
   and hand each to the registered file interceptor. Up to 8 blobs download at a time, while the
   interceptor is still called once at a time and in ranked order.

`MaxFileSize` is enforced at three points on the way to the interceptor, in this order:

1. **Before the request**, against `CandidateFile.Size`. On a walk that size comes straight from
   the tree, so an oversized blob is never downloaded at all; the entry is dropped during ranking
   and no request is made for it.
2. **While reading the response**, which stops at a ceiling derived from `MaxFileSize` (the
   base64 inflation, its line breaks and a small envelope allowance). A blob that runs past it is
   abandoned mid-read rather than buffered whole, and refused with `ErrInvalidSizeFile`.
3. **After decoding**, against the decoded byte length, refused with `ErrInvalidSizeFile` before
   anything reaches the interceptor.

Which gate a caller driving `ListInterestingFiles` and then `FetchCandidates` hits depends on one
thing: whether the candidate still carries its `Size`. `ListInterestingFiles` fills it in from the
tree entry, so a caller that hands the listing straight back - or round-trips it as JSON, where a
non-zero size survives - is refused by the first gate before any request, exactly as on a walk.
`CandidateFile.Size` is `json:"size,omitempty"`, so only a client that rebuilds candidates from a
subset of the fields, path and SHA say, loses it: then it arrives as `0`, the pre-request check
passes, and a modestly oversized blob still fits inside the read ceiling - whose envelope
allowance is a fixed number of bytes, so it is generous at small limits. The decoded check is
what holds the limit for that caller.

The crawl is **opt-in**: without `UseGithubAPI()` no request is made to the GitHub API, the walk
clones as it always has, and the ranking, the selective fetch and the repository-relative
`File.Path` are all out of the picture.

Two changes do reach callers that never opt in, and both are deliberate:

- **An explicitly set `Branch` now reaches the clone.** `Branch` used to be recorded and never put
  on the clone options, so only `ReferenceName` selected anything; the clone always took the
  remote's default branch. Fixing that gap is the first acceptance criterion of
  [#1119](https://github.com/meshery/meshkit/issues/1119), and it means `Branch("master")` against
  a repository whose default branch is `main` now fails with `ErrCloningRepo` instead of quietly
  cloning `main`. A caller that never calls `Branch` is unaffected.
- **A symlink is read only while its target resolves inside the repository copy.** On every clone,
  a link whose target lands outside the copy the walk made, or that cannot be resolved, is passed
  over silently rather than read through. Links that stay inside the copy are followed as before.

### When the crawl falls back to go-git

`WalkContext` falls back to the clone walk whenever the API route cannot answer completely:

- **The host is not github.com.** The host is read from the configured `BaseURL`, never assumed,
  so GitHub Enterprise, GitLab, Bitbucket and `file://` URLs keep taking the clone path.
- **The tree came back truncated.** GitHub caps a single tree response; `truncated: true` means
  the listing is incomplete, so a clone reads the repository instead.
- **A directory interceptor is registered.** Directory interception needs a directory that
  exists, which the API route never produces, so the walk clones.

**Only the truncated-tree clone is filtered**, because it is the only one standing in for a Trees
walk of the same repository. There the clone route runs the same classifier the ranking uses, so
`.md`, `.txt`, `.sh` and extensionless files never reach the interceptor, it skips a file over
`MaxFileSize` instead of failing the walk, and it skips symlinks outright as the API route does,
which never offers one. What the caller gets is the same filtered, size-bounded **set** - not the
same order: the clone delivers in `filepath.WalkDir`/`os.ReadDir` lexical order, never in score
order, so an interceptor must not assume `Chart.yaml` arrives before `values.yaml`.

**The other two clones are not filtered.** A non-github.com host and a registered directory
interceptor take the clone route on their own terms, not as a substitute for a Trees walk, so they
behave exactly as they always have even when the caller enabled `UseGithubAPI()`: every file under
`Root`, in the order it always arrived, with the usual oversize error rather than a silent skip.
Opting in does change the file paths the interceptor is handed on any clone - see **Paths** below.

Symlinks are followed on an unfiltered clone, with one
limit - a link is read only while its target resolves inside the repository copy the walk made. A
link whose target lands outside that copy, or that cannot be resolved at all, is passed over
silently; the walk continues and nothing else changes. Targets are resolved with `lstat`/`readlink`
alone, so a file that will not be read is never opened.

**A `Root` that names nothing fails on either route.** The clone route stats the path and fails
with `ErrCloningRepo`; the API route proves the root against the tree - an entry at `Root` itself
or anything below it - and fails with `ErrRootNotFound`, rather than importing no files and
calling that a success. A truncated tree cannot show that a root is absent, so that case is left
to the clone it falls back to. An unset `Root`, `Root("")` and `Root("/")` all mean the whole
repository and are never checked.

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

`MaxFileSize(0)` is rejected by both (`ErrInvalidSizeFile`) exactly as it is on a walk, rather
than answered with an empty listing or an import of whatever happens to fit. `FetchCandidates`
also refuses an individual candidate that turns out to exceed `MaxFileSize` (`ErrInvalidSizeFile`
again), at whichever of the three points above catches it - which for a selection that lost its
`Size` in transit is after the download, not before it.

`FetchCandidates` also refuses a walker with no file interceptor registered
(`ErrNoFileInterceptor`): it downloads one file per candidate and would have nowhere to hand them,
so an import of nothing is reported as the mistake it is rather than as success. `WalkContext`'s
own API route is deliberately quieter about this - with no interceptor it has nothing worth
downloading, so it fetches nothing and moves on.

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
  the commits endpoint answers with that branch's head commit. `NewGit()`'s `"master"` default is
  only reached once `Branch` has been called, so it is never forced onto either of `Git`'s routes.
- **`Github` always sends its branch.** The Contents walker defers nothing: `NewGithub()` sets
  `"main"` and every request carries `?ref=<branch>`, so a caller that never calls `Branch` asks
  for `main` whatever the repository's default branch is, and a repository that defaults to
  something else answers 404. Call `Branch` explicitly when driving `Github` from a connection
  that does not carry one.
- **Context.** `WalkContext`, `ListInterestingFiles` and `FetchCandidates` all take a context;
  `Walk()` delegates with `context.Background()`. `Timeout(d)` bounds a whole traversal. A
  traversal cut short by a deadline or a cancellation fails: `Github.WalkContext` fans out one
  request per entry, and once the fan-out has drained an ended context is returned as an error
  rather than a partial import reported as a success - that is the one case where the directory
  interceptor is skipped. Any **individual** node that cannot be listed or decoded is logged and
  passed over instead, and the rest of the walk still delivers: GitHub answers a submodule or a
  symlink path with a single JSON object rather than an array, so those nodes fail to decode and
  are skipped, and a 404 or a rate limit on one subtree does not abort the import. A caller that
  needs to know a node was missed reads it off the interceptors it was handed, not off the
  returned error.
- **Progress.** `RegisterProgressHook` receives `ProgressUpdate` values as the walk moves
  through `resolve-ref`, `list-tree`, `rank`, `fetch-blob` and `clone`.
- **Paths.** Under `UseGithubAPI()` every **file** path handed to an interceptor is
  repository-relative - on the API route and on the clone it falls back to - so one import handles
  one kind of path. A `File.Path` **must not be opened from the filesystem**: on the API route
  nothing is written to disk at all, and on the clone route the temporary clone is deleted when
  the walk returns. A caller that reads `File.Path` back off disk, as `generators/github` does,
  should **not** opt in. Callers that never opted in keep the absolute path into the (temporary)
  clone they have always received, byte for byte.
- **Directory paths.** `Directory.Path` is always the real path on disk, for every caller. Only
  the clone route ever produces a `Directory` - a walk with a directory interceptor registered
  declines the API route precisely to get one - and a `Directory` carries no content, so opening
  it is the only thing an interceptor can do with it. It is valid for as long as the walk runs and
  is removed with the clone once `Walk` returns, so anything a directory interceptor needs from it
  (`helm.ConvertToK8sManifest`, say) has to happen inside the interceptor.
- **Auth.** `Token(t)` threads a GitHub App or OAuth token onto the API calls as a bearer token
  and onto the clone as `x-access-token` basic auth, for private repositories and the
  authenticated rate limit. The token is never logged, never placed in an error message and
  never reported through a progress hook. Because a token makes private repositories clonable,
  the temporary clone directory is created `0700` before go-git checks anything out, so a private
  repository's working copy is never readable by other local users; that holds for every clone,
  token or not.
