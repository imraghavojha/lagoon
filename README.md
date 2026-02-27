# lagoon

reproducible sandboxed shell environments. no docker. no root. no daemons.

you describe the tools you need. anyone on any matching linux machine gets an identical shell with exactly those tools — same versions, guaranteed, every time.

```
lagoon init     # pick your packages
lagoon shell    # enter the sandbox
lagoon clean    # wipe the cache
```

---

## how it works

lagoon writes a `lagoon.toml` with your packages and a pinned nixpkgs commit. you commit that file. anyone who runs `lagoon shell` gets the exact same environment — same binary, same version, same nix store path.

bwrap creates the sandbox: your project directory is mounted at `/workspace`, the nix store is read-only, everything else is empty. network is off unless you asked for it. when you exit, nothing persists.

---

## install

**requirements:** linux (arm64 or amd64), bubblewrap, nix

```bash
# install bubblewrap and nix if you don't have them
sudo apt install bubblewrap
sh <(curl -L https://nixos.org/nix/install) --no-daemon && source ~/.nix-profile/etc/profile.d/nix.sh

# install lagoon
curl -fsSL https://raw.githubusercontent.com/imraghavojha/lagoon/main/install.sh | bash
```

---

## usage

```bash
# in your project directory
lagoon init        # interactive setup — pick packages, commit the result
lagoon shell       # enter the sandbox (first run downloads packages)
lagoon clean       # remove cached shell.nix for this project
```

inside the sandbox:
- your project is at `/workspace` and you start there
- `HOME` is `/home` (ephemeral tmpfs — nothing persists between sessions)
- only the packages you asked for are on `PATH`
- network is off by default (set `profile = "network"` in lagoon.toml to enable)

---

## lagoon.toml

```toml
packages = ["python311", "ffmpeg"]
nixpkgs_commit = "26eaeac4e409d7b5a6bf6f90a2a2dc223c78d915"
nixpkgs_sha256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
profile = "minimal"   # or "network"
```

`lagoon init` writes this for you. the nixpkgs pin is hardcoded in the binary — you never need to find or set it manually. search for package names at [search.nixos.org/packages](https://search.nixos.org/packages).

---

## target platforms

primary: arm64 linux (raspberry pi 4/5, ubuntu 22.04+)
secondary: x86-64 linux

first run on arm may take 10–60 minutes if packages aren't in the binary cache. this only happens once.

---

## build from source

```bash
git clone https://github.com/imraghavojha/lagoon
cd lagoon
go build -o lagoon .
```
