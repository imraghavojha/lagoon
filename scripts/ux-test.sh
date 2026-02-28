#!/usr/bin/env bash
set -e

# Setup environment
export PATH=$PATH:/usr/local/go/bin
[ -f "$HOME/.nix-profile/etc/profile.d/nix.sh" ] && source "$HOME/.nix-profile/etc/profile.d/nix.sh"

cd /home/ubuntu.linux/lagoon
# echo "Building lagoon..."
# go build -o lagoon .

echo "Setting up test directory..."
TEST_DIR="/tmp/lagoon-ux-test"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

# Create a lagoon.toml with 'hello' (small package)
cat > lagoon.toml << EOF
packages = ["hello"]
nixpkgs_commit = "de21e7685d03f8a2e0b4af7a1fb0e8a9bf99d9b5"
nixpkgs_sha256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
profile = "minimal"
on_enter = "echo 'Entering sandbox...'"
EOF

LAGOON="/home/ubuntu.linux/lagoon/lagoon"

echo ""
echo "--- UX: lagoon status (not cached) ---"
$LAGOON status

echo ""
echo "--- UX: lagoon run (cold start + build) ---"
# lagoon run hello should trigger the build and then execute hello
$LAGOON run hello

echo ""
echo "--- UX: lagoon status (cached) ---"
$LAGOON status

echo ""
echo "--- UX: lagoon run (warm start) ---"
$LAGOON run hello

echo ""
echo "--- UX: lagoon run with spaces in args ---"
$LAGOON run echo "hello world"

echo ""
echo "--- UX: lagoon verify ---"
$LAGOON verify

echo ""
echo "--- UX: lagoon export ---"
$LAGOON export > test.nar
echo "Exported NAR size: $(du -h test.nar | cut -f1)"

echo ""
echo "--- UX: lagoon clean ---"
$LAGOON clean
$LAGOON status
