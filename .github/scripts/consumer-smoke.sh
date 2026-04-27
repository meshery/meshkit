#!/usr/bin/env bash
set -euo pipefail

WORKDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

cat >"$WORKDIR/go.mod" <<MODULE
module meshkit-smoke

go 1.25.5

require github.com/meshery/meshkit v0.0.0

replace github.com/meshery/meshkit => ${GITHUB_WORKSPACE:-$PWD}
MODULE

cat >"$WORKDIR/smoke_test.go" <<'TEST'
package smoke

import (
    "testing"

    mkerrors "github.com/meshery/meshkit/errors"
)

func TestMeshkitAsDependency(t *testing.T) {
    err := mkerrors.New(
        "11001",
        mkerrors.Alert,
        []string{"Dependency smoke test"},
        []string{"MeshKit consumer build works"},
        []string{"None"},
        []string{"None"},
    )

    if mkerrors.GetCode(err) != "11001" {
        t.Fatalf("expected error code 11001")
    }
}
TEST

cd "$WORKDIR"
go mod tidy
go test ./... -v
