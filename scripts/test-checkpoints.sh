#!/usr/bin/env bash
# runs checkpoint tests inside the vm.
# must run setup-vm.sh first, then open a fresh shell so nix and go are on PATH.
# usage: bash /home/ubuntu/lagoon/scripts/test-checkpoints.sh

set -e

WORKDIR="/home/ubuntu/lagoon"
LOGFILE="$WORKDIR/scripts/test.log"
exec > >(tee "$LOGFILE") 2>&1

source "$HOME/.nix-profile/etc/profile.d/nix.sh" 2>/dev/null || true
export PATH=$PATH:/usr/local/go/bin

echo "=== lagoon checkpoint tests ==="
echo "started: $(date)"
echo "arch: $(uname -m)"

# --- build the binary ---
echo ""
echo "--- building lagoon ---"
cd "$WORKDIR"
go build -o /home/ubuntu/lagoon-bin .
echo "build: ok"

# --- checkpoint 1: help text ---
echo ""
echo "--- checkpoint 1: help text ---"
/home/ubuntu/lagoon-bin --help | grep -q "lagoon" && echo "PASS: help text contains 'lagoon'" || echo "FAIL: help text missing"
/home/ubuntu/lagoon-bin --help | grep -q "init" && echo "PASS: init command listed" || echo "FAIL: init command missing"
/home/ubuntu/lagoon-bin --help | grep -q "shell" && echo "PASS: shell command listed" || echo "FAIL: shell command missing"
/home/ubuntu/lagoon-bin --help | grep -q "clean" && echo "PASS: clean command listed" || echo "FAIL: clean command missing"

# --- checkpoint 2: init command ---
echo ""
echo "--- checkpoint 2: init command ---"
TESTDIR=$(mktemp -d)
cd "$TESTDIR"

# simulate user typing "cowsay" then "n" for network
printf "cowsay\nn\n" | /home/ubuntu/lagoon-bin init

if [ -f lagoon.toml ]; then
    echo "PASS: lagoon.toml created"
    echo "contents:"
    cat lagoon.toml
else
    echo "FAIL: lagoon.toml not created"
fi

# test overwrite prompt — answer n, should not overwrite
ORIG=$(cat lagoon.toml)
printf "n\n" | /home/ubuntu/lagoon-bin init 2>&1 | grep -q "not overwriting" && echo "PASS: overwrite declined" || echo "FAIL: overwrite prompt broken"
[[ "$(cat lagoon.toml)" == "$ORIG" ]] && echo "PASS: file unchanged after declining" || echo "FAIL: file changed despite declining"

# --- checkpoint 3: preflight checks ---
echo ""
echo "--- checkpoint 3: preflight checks ---"
cd "$TESTDIR"

# all tools present — should get past preflight (will fail on missing toml, that is fine)
OUTPUT=$(/home/ubuntu/lagoon-bin shell 2>&1 || true)
if echo "$OUTPUT" | grep -q "no lagoon.toml"; then
    echo "PASS: preflight passed (got expected 'no lagoon.toml' error)"
elif echo "$OUTPUT" | grep -q "bubblewrap not found\|nix not found\|user namespaces"; then
    echo "FAIL: preflight check failed when it should pass"
    echo "output: $OUTPUT"
else
    echo "INFO: shell output: $OUTPUT"
fi

# test bwrap missing
sudo mv "$(which bwrap)" /usr/bin/bwrap.bak
OUTPUT=$(/home/ubuntu/lagoon-bin shell 2>&1 || true)
sudo mv /usr/bin/bwrap.bak "$(which bwrap)" 2>/dev/null || sudo mv /usr/bin/bwrap.bak /usr/bin/bwrap
if echo "$OUTPUT" | grep -qi "bubblewrap not found"; then
    echo "PASS: bwrap missing error shown"
else
    echo "FAIL: expected 'bubblewrap not found', got: $OUTPUT"
fi

# --- checkpoint 4: shell.nix generation ---
echo ""
echo "--- checkpoint 4: shell.nix generation ---"
cd "$TESTDIR"

# read the real pin values from setup output
if [ -f "$WORKDIR/scripts/nixpkgs-pin.txt" ]; then
    source "$WORKDIR/scripts/nixpkgs-pin.txt"
    echo "using pin: $NIXPKGS_COMMIT"
    echo "sha256: $NIXPKGS_SHA256"

    cat > lagoon.toml << EOF
packages = ["cowsay"]
nixpkgs_commit = "$NIXPKGS_COMMIT"
nixpkgs_sha256 = "$NIXPKGS_SHA256"
profile = "minimal"
EOF

    # run shell briefly — it should create shell.nix then try to resolve
    # we just check the shell.nix was written correctly, not that nix resolves
    /home/ubuntu/lagoon-bin shell 2>&1 | head -3 || true

    NIXFILE=$(find ~/.cache/lagoon -name shell.nix 2>/dev/null | head -1)
    if [ -n "$NIXFILE" ]; then
        echo "PASS: shell.nix created at $NIXFILE"
        if grep -q "cowsay" "$NIXFILE" && grep -q "bash" "$NIXFILE" && grep -q "coreutils" "$NIXFILE"; then
            echo "PASS: shell.nix has cowsay, bash, coreutils"
        else
            echo "FAIL: shell.nix missing expected packages"
        fi
        if grep -q "$NIXPKGS_COMMIT" "$NIXFILE"; then
            echo "PASS: shell.nix has pinned commit"
        else
            echo "FAIL: shell.nix missing pinned commit"
        fi
    else
        echo "FAIL: shell.nix not created in cache"
    fi
else
    echo "SKIP: nixpkgs-pin.txt not found, run setup-vm.sh first"
fi

# --- checkpoint 5: nix-shell invocation and error handling ---
echo ""
echo "--- checkpoint 5: nix-shell and error handling ---"
cd "$TESTDIR"

# test bad package name gives clean error
cat > lagoon.toml << EOF
packages = ["thisdoesnotexist12345"]
nixpkgs_commit = "$NIXPKGS_COMMIT"
nixpkgs_sha256 = "$NIXPKGS_SHA256"
profile = "minimal"
EOF

OUTPUT=$(/home/ubuntu/lagoon-bin shell 2>&1 || true)
if echo "$OUTPUT" | grep -qi "package not found\|thisdoesnotexist"; then
    echo "PASS: clean error for unknown package"
else
    echo "INFO: error output for bad package:"
    echo "$OUTPUT" | head -20
fi

echo ""
echo "=== tests complete ==="
echo "finished: $(date)"
echo ""
echo "checkpoint 5 full nix resolution and checkpoint 6 sandbox need:"
echo "  run: bash /home/ubuntu/lagoon/scripts/test-sandbox.sh"
