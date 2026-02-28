# Lagoon

Reproducible sandboxed shell environments. No Docker. No root. No daemons.

You describe the tools you need. Anyone on any matching Linux machine gets an identical shell with exactly those tools — same versions, guaranteed, every time.

```
lagoon init     # search and pick packages
lagoon shell    # enter the sandbox
lagoon clean    # wipe the cache
lagoon export > myenv.nar   # snapshot for offline use
lagoon import myenv.nar     # restore on air-gapped machine
```

---

## How it works

Lagoon writes a `lagoon.toml` with your packages and a pinned nixpkgs commit. You commit that file. Anyone who runs `lagoon shell` gets the exact same environment — same binary, same version, same nix store path.

bwrap creates the sandbox: your project directory is mounted at `/workspace`, the nix store is read-only, everything else is empty. Network is off unless you asked for it. When you exit, nothing persists.

---

## Install

**Requirements:** Linux (arm64 or amd64), bubblewrap, nix

```bash
# Install bubblewrap and nix if you don't have them
sudo apt install bubblewrap
sh <(curl -L https://nixos.org/nix/install) --no-daemon && source ~/.nix-profile/etc/profile.d/nix.sh

# Install lagoon
curl -fsSL https://raw.githubusercontent.com/imraghavojha/lagoon/main/install.sh | bash
```

---

## Usage

```bash
# In your project directory
lagoon init        # interactive setup — search packages live, commit the result
lagoon shell       # enter the sandbox (first run downloads packages)
lagoon shell -m 512m   # limit memory to 512 MiB (uses systemd-run)
lagoon clean       # remove cached environment for this project
lagoon status      # show whether the environment is cached
lagoon export > myenv.nar   # export full nix closure for offline transfer
lagoon import myenv.nar     # import on an air-gapped machine
```

Inside the sandbox:
- Your project is at `/workspace` and you start there
- `HOME` is `/home` (ephemeral tmpfs — nothing persists between sessions)
- Only the packages you asked for are on `PATH`
- Network is off by default (set `profile = "network"` in lagoon.toml to enable)

---

## lagoon.toml

```toml
packages = ["python311", "ffmpeg"]
nixpkgs_commit = "26eaeac4e409d7b5a6bf6f90a2a2dc223c78d915"
nixpkgs_sha256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
profile = "minimal"   # or "network"
```

`lagoon init` writes this for you. The nixpkgs pin is hardcoded in the binary — you never need to find or set it manually. `lagoon init` searches [search.nixos.org](https://search.nixos.org/packages) live as you type.

---

## Memory limits

On shared machines (e.g., a Raspberry Pi running multiple student environments), you can cap each sandbox's memory:

```bash
lagoon shell --memory 512m   # 512 MiB
lagoon shell -m 2g            # 2 GiB
lagoon run -m 256m python3 script.py
```

This wraps bwrap with `systemd-run --scope -p MemoryMax=...`. Requires systemd (standard on Ubuntu 22.04+).

---

## Offline / air-gapped deployments

Export an environment on a connected machine, then import on one with no internet:

```bash
# On a machine with internet:
lagoon shell       # build and cache the environment first
lagoon export > myenv.nar

# Copy myenv.nar to the air-gapped machine, then:
lagoon import myenv.nar
lagoon shell       # works fully offline
```

---

## Target platforms

Primary: arm64 Linux (Raspberry Pi 4/5, Ubuntu 22.04+)
Secondary: x86-64 Linux

First run on ARM may take 10–60 minutes if packages aren't in the binary cache. This only happens once.

---

## Build from source

```bash
git clone https://github.com/imraghavojha/lagoon
cd lagoon
go build -o lagoon .
```
