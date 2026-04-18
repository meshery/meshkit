# Sync Go mods

go run github.com/meshery/meshkit/cmd/syncmodutil < path to src go mod >  < path to destination go mod >

Pass `--err` as a third argument to fail (non-zero exit) when the destination
`require` block already contains a different version than the source for any
shared module, instead of rewriting it in place.

## What it does

1. Rewrites every shared `require` line in the destination `go.mod` to match
   the source's version.
2. Appends a `replace` block that pins every module in the source module
   graph to the source's exact selected version. Source-declared `replace`
   directives are preserved verbatim and take precedence over pinning.

Step 2 is what makes the result safe for **Go plugin builds**. Without it, a
`go mod tidy` run after sync can upgrade transitive dependencies past what
the host binary is linked against — causing `plugin.Open` to fail at runtime
with `plugin was built with a different version of package ...`. Because
`replace` directives override `require` resolution, the destination module
is guaranteed to compile against the same versions as the source no matter
what `tidy` does afterwards.

# Caveats
1. Always perform a build test of destination go module after syncing to make sure that nothing breaks.
2. If destination go module relied on a specific version having similar API contract but different internal logic, then you may be in trouble.
3. The emitted `replace` block is intentionally broad (it pins every module
   in the source graph). Unused pins are harmless — `go mod tidy` keeps them
   but they have no effect on modules the destination does not import.


## Flow

![](./go-mod-sync-flow.svg)
