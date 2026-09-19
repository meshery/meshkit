# Canvas-first GitHub onboarding — meshkit follow-on

Primary design research (Canvas UX + Cloud APIs + Git crawl):

**https://github.com/layer5labs/meshery-extensions/pull/4366**

Cloud pointer: **https://github.com/layer5io/meshery-cloud/pull/6115**

## Why meshkit cares

Import performance and file selection quality depend on meshkit’s Git walkers (`go-git` full shallow clone vs GitHub Trees/Contents). Research recommends a **hybrid**: GitHub → Trees + selective blobs + interesting-file ranking; keep `go-git` for non-GitHub / truncated-tree fallback.

## meshkit-owned remediation

| Priority | Item |
|----------|------|
| P0 | Fix branch/ref wiring on clone options; timeouts; progress hooks |
| P2 | Route GitHub design import through Trees → filter → selective fetch |
| P2 | Wire connection OAuth/App token into walkers (private repos + rate limits) |
| P2 | Ranked interesting-file list API (Helm / K8s / Compose / designs) before blob download |
| Later | Sparse/partial clone for non-GitHub large repos; optional GitLab/Bitbucket adapters |

## Non-goals

No production code in this pointer doc. Implementation PRs should cite extensions#4366 and the Git-layer appendix therein.
