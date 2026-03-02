#!/usr/bin/env bash
# run_vm_tests.sh — orchestrate lagoon VM tests via Lima.
#
# usage:
#   bash tests/vm/run_vm_tests.sh arm         # run on ARM Lima VM
#   bash tests/vm/run_vm_tests.sh x86         # run on x86 Lima VM
#   bash tests/vm/run_vm_tests.sh arm --e2e   # include slow nix builds
#   bash tests/vm/run_vm_tests.sh x86 --e2e
#
# requirements: limactl in PATH, VM already started.
# start arm VM:  limactl start --name lagoon-arm scripts/lagoon-vm.yaml
# start x86 VM:  limactl start --name lagoon-x86 --arch x86_64 scripts/lagoon-vm.yaml
set -euo pipefail

ARCH="${1:-arm}"
E2E=0
for arg in "$@"; do
  [ "$arg" = "--e2e" ] && E2E=1
done

case "$ARCH" in
  arm)  VM_NAME="lagoon-arm"  ;;
  x86)  VM_NAME="lagoon-x86"  ;;
  *)    echo "usage: $0 arm|x86 [--e2e]"; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SUITE="$SCRIPT_DIR/test_suite.sh"
BINARY="$REPO_ROOT/lagoon"

# check limactl is available
if ! command -v limactl &>/dev/null; then
  echo "ERROR: limactl not found — install Lima to run VM tests"
  echo "  brew install lima  (macOS)"
  exit 1
fi

# check VM is running
if ! limactl list --format json 2>/dev/null | grep -q "\"name\":\"$VM_NAME\""; then
  echo "ERROR: VM '$VM_NAME' not found"
  echo "  start it with: limactl start --name $VM_NAME scripts/lagoon-vm.yaml"
  [ "$ARCH" = "x86" ] && echo "  (add --arch x86_64 for x86 VM)"
  exit 1
fi

VM_STATUS=$(limactl list --format json 2>/dev/null | python3 -c "
import json,sys
data=json.load(sys.stdin) if sys.stdin.read(1)=='{' else [json.loads(l) for l in sys.stdin]
" 2>/dev/null || limactl list "$VM_NAME" 2>/dev/null | awk 'NR>1{print $2}')

echo "=== running lagoon VM tests on $VM_NAME ($ARCH) ==="
echo ""

# helper: run a command inside the VM
vm() { limactl shell "$VM_NAME" -- "$@"; }

# step 1: build the binary for the target arch inside the VM
echo "--- building lagoon binary inside $VM_NAME ---"
if [ -f "$BINARY" ]; then
  SSH_CONFIG=$(limactl show-ssh --format config "$VM_NAME" 2>/dev/null || true)
  if [ -n "$SSH_CONFIG" ]; then
    scp -F <(echo "$SSH_CONFIG") "$BINARY" "lima-${VM_NAME}:/usr/local/bin/lagoon" 2>/dev/null || \
      vm sudo cp /tmp/lagoon /usr/local/bin/lagoon 2>/dev/null || true
  fi
else
  echo "INFO: no pre-built binary at $REPO_ROOT/lagoon — building inside VM"
  vm bash -c "cd /tmp && git clone --depth 1 file:///$(basename $REPO_ROOT) lagoon-src 2>/dev/null || true"
fi

# step 2: copy and run the test suite
echo "--- copying test suite ---"
SUITE_DEST="/tmp/lagoon_test_suite_$$.sh"

if limactl list "$VM_NAME" &>/dev/null; then
  # copy test suite into VM
  SSH_CONFIG=$(limactl show-ssh --format config "$VM_NAME" 2>/dev/null || echo "")
  if [ -n "$SSH_CONFIG" ]; then
    scp -F <(echo "$SSH_CONFIG") "$SUITE" "lima-${VM_NAME}:${SUITE_DEST}" 2>/dev/null || {
      # fallback: pipe the script content via limactl shell
      vm bash -c "cat > $SUITE_DEST" < "$SUITE"
    }
  else
    vm bash -c "cat > $SUITE_DEST" < "$SUITE"
  fi
fi

echo "--- running test suite ---"
echo ""

E2E_FLAG=""
[ "$E2E" = "1" ] && E2E_FLAG="LAGOON_E2E=1"

# run the suite and stream output
vm bash -c "$E2E_FLAG bash $SUITE_DEST" 2>&1
EXIT_CODE=$?

# clean up
vm bash -c "rm -f $SUITE_DEST" 2>/dev/null || true

echo ""
if [ "$EXIT_CODE" -eq 0 ]; then
  echo "=== ALL TESTS PASSED on $VM_NAME ($ARCH) ==="
else
  echo "=== SOME TESTS FAILED on $VM_NAME ($ARCH) — exit code $EXIT_CODE ==="
fi

exit "$EXIT_CODE"
