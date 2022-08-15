# Sync Go mods

go run github.com/layer5io/meshkit/cmd/syncmodutil < path to src go mod >  < path to destination go mod >


# Caviats
1. Always perform a build test of destination go module after syncing to make sure that nothing breaks.
2. If destination go module relied on a specific version having similar API contract but different internal logic, then you may be in trouble.