#!/usr/bin/env bash
# tests the example stacks end-to-end inside the vm.
# usage: bash scripts/test-real-repos.sh <path-to-lagoon-binary>
# run from the repo root.

set -e

BIN=${1:-lagoon}
REPO=$(cd "$(dirname "$0")/.." && pwd)
PASS=0
FAIL=0

run_test() {
    local name=$1 dir=$2 cmd=$3
    echo ""
    echo "--- $name ---"
    cd "$dir"
    if OUTPUT=$("$BIN" run "$cmd" 2>&1); then
        echo "$OUTPUT"
        echo "PASS: $name"
        PASS=$((PASS + 1))
    else
        echo "$OUTPUT"
        echo "FAIL: $name"
        FAIL=$((FAIL + 1))
    fi
}

run_test "python" "$REPO/examples/python" "python3 hello.py"
run_test "node"   "$REPO/examples/node"   "node hello.js"
run_test "ruby"   "$REPO/examples/ruby"   "ruby hello.rb"

echo ""
echo "=== results: $PASS passed, $FAIL failed ==="
[ $FAIL -eq 0 ]
