#!/usr/bin/env bash
set -euo pipefail

# 11 cases test script for errorutil pre-commit hook
echo "Setting up test environment..."

ROOT_DIR=$(pwd)
TEST_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_DIR"' EXIT

# Fast clone to get the exact toolchain and go.mod
git clone -q --shared "$ROOT_DIR" "$TEST_DIR"
cp -r "$ROOT_DIR/.githooks" "$TEST_DIR/"
cd "$TEST_DIR"
git config user.name "Test User"
git config user.email "test@example.com"
git config core.hooksPath .githooks

# Set up baseline component info
mkdir -p helpers pkg
cat > helpers/component_info.json <<EOF
{
  "name": "meshkit",
  "type": "library",
  "next_error_code": 11000
}
EOF
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
EOF

git add helpers/component_info.json pkg/error.go
git commit --no-verify -m "baseline" -q

# Run cases

run_hook() {
    # runs the hook directly so we can check exit code without needing an actual commit
    ./.githooks/pre-commit
}

assert_pass() {
    if ! "$@"; then
        echo "FAIL: expected pass: $*"
        exit 1
    fi
}

assert_fail() {
    if "$@"; then
        echo "FAIL: expected fail: $*"
        exit 1
    fi
}

echo "1. Nothing staged"
assert_pass run_hook

echo "2. replace_me staged"
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "replace_me"
EOF
git add pkg/error.go
assert_pass run_hook

echo "3. Real local allocation staged"
make errorutil >/dev/null 2>&1
git add pkg/error.go helpers/component_info.json
assert_pass run_hook

echo "4. Hand-typed integer staged"
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "meshkit-11005"
EOF
git add pkg/error.go
assert_fail run_hook

echo "5. Staged clean + unstaged dirty"
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "replace_me"
EOF
git add pkg/error.go
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "meshkit-11005"
EOF
assert_pass run_hook

echo "6. Staged dirty + unstaged clean"
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "meshkit-11005"
EOF
git add pkg/error.go
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "replace_me"
EOF
assert_fail run_hook

echo "7. Unrelated file only staged"
git reset --hard HEAD >/dev/null 2>&1
echo "foo" > README.md
git add README.md
assert_pass run_hook

echo "8. Multiple error.go staged, one bad"
git reset --hard HEAD >/dev/null 2>&1
mkdir -p otherpkg
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "replace_me"
EOF
cat > otherpkg/error.go <<EOF
package otherpkg
const ErrBadCode = "meshkit-11005"
EOF
git add pkg/error.go otherpkg/error.go
assert_fail run_hook

echo "9. Deleted error.go staged"
git reset --hard HEAD >/dev/null 2>&1
git rm -q pkg/error.go
assert_pass run_hook

echo "10. Index not mutated"
git reset --hard HEAD >/dev/null 2>&1
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "replace_me"
EOF
git add pkg/error.go
HASH_BEFORE=$(git ls-files -s pkg/error.go | awk '{print $2}')
run_hook
HASH_AFTER=$(git ls-files -s pkg/error.go | awk '{print $2}')
if [ "$HASH_BEFORE" != "$HASH_AFTER" ]; then
    echo "FAIL: index mutated"
    exit 1
fi

echo "11. git commit -a"
git reset --hard HEAD >/dev/null 2>&1
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "meshkit-11005"
EOF
# Hook invoked via git commit -a
if git commit -a -m "should fail" >/dev/null 2>&1; then
    echo "FAIL: git commit -a should have failed"
    exit 1
fi

echo "12. Repeated runs (reliability / no fail-open skips)"
git reset --hard HEAD >/dev/null 2>&1
cat > pkg/error.go <<EOF
package pkg
const ErrCode = "meshkit-10001"
const ErrNewCode = "replace_me"
EOF
git add pkg/error.go
for i in $(seq 1 5); do
    assert_pass run_hook
done

echo "13. cygpath execution path verification"
MOCK_BIN=$(mktemp -d)
cat > "$MOCK_BIN/cygpath" <<'EOF'
#!/bin/sh
echo "$2"
EOF
chmod +x "$MOCK_BIN/cygpath"
(
  export PATH="$MOCK_BIN:$PATH"
  assert_pass run_hook
)
rm -rf "$MOCK_BIN"

echo "ALL TESTS PASSED!"
