#!/usr/bin/env bash
# sets up the vm for lagoon development and testing.
# run this inside the vm: bash /home/ubuntu/lagoon/scripts/setup-vm.sh

set -e

LOGFILE="/home/ubuntu/lagoon/scripts/setup.log"
exec > >(tee "$LOGFILE") 2>&1

echo "=== lagoon vm setup ==="
echo "started: $(date)"

# --- apt packages ---
echo ""
echo "--- installing apt packages ---"
sudo apt-get update -q
sudo apt-get install -y -q bubblewrap git curl wget

echo "bwrap version: $(bwrap --version)"

# --- nix ---
echo ""
echo "--- installing nix ---"
if command -v nix-shell &>/dev/null; then
    echo "nix already installed: $(nix-shell --version)"
else
    # pipe install through bash — --no-daemon keeps it simple for a test vm
    sh <(curl -L https://nixos.org/nix/install) --no-daemon
fi

# source nix profile for the rest of this script
if [ -f "$HOME/.nix-profile/etc/profile.d/nix.sh" ]; then
    source "$HOME/.nix-profile/etc/profile.d/nix.sh"
fi

echo "nix-shell version: $(nix-shell --version)"

# --- go ---
echo ""
echo "--- installing go ---"
if command -v go &>/dev/null; then
    echo "go already installed: $(go version)"
else
    GOARCH=$(uname -m)
    if [ "$GOARCH" = "aarch64" ]; then
        GOTAR="go1.21.13.linux-arm64.tar.gz"
    else
        GOTAR="go1.21.13.linux-amd64.tar.gz"
    fi
    wget -q "https://go.dev/dl/$GOTAR"
    sudo tar -C /usr/local -xzf "$GOTAR"
    rm "$GOTAR"
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
fi

export PATH=$PATH:/usr/local/go/bin
echo "go version: $(go version)"

# --- get real nixpkgs sha256 ---
echo ""
echo "--- fetching nixpkgs pin sha256 (this takes a moment) ---"
# using a known-good nixpkgs-unstable commit from 2024-12-01
NIXPKGS_COMMIT="de21e7685d03f8a2e0b4af7a1fb0e8a9bf99d9b5"
echo "commit: $NIXPKGS_COMMIT"

SHA256=$(nix-prefetch-url --unpack \
    "https://github.com/NixOS/nixpkgs/archive/${NIXPKGS_COMMIT}.tar.gz" 2>/dev/null)

echo "sha256: $SHA256"

# write the real values to a file so the host can read them
cat > /home/ubuntu/lagoon/scripts/nixpkgs-pin.txt << EOF
NIXPKGS_COMMIT=${NIXPKGS_COMMIT}
NIXPKGS_SHA256=${SHA256}
EOF

echo ""
echo "=== setup complete ==="
echo "finished: $(date)"
echo ""
echo "--- checkpoint 0 verification ---"
bwrap --version
nix-shell --version
go version
uname -m
cat /proc/sys/kernel/unprivileged_userns_clone 2>/dev/null && echo "(userns ok)" || echo "(userns file missing — that is fine, means enabled by default)"

echo ""
echo "pin values written to /home/ubuntu/lagoon/scripts/nixpkgs-pin.txt"
echo "run checkpoint tests next: bash /home/ubuntu/lagoon/scripts/test-checkpoints.sh"
