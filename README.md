# Lagoon

Lagoon is a beautiful CLI for reproducible dev environments and small local runtimes — on Linux **and** macOS.

It gives you the parts of Docker developers usually want locally — repeatable shells, small service stacks, offline portability, and an escape hatch to Docker — with the lightest runtime each platform offers:

- **Linux** — Nix + bubblewrap. No daemon, no images, no Kubernetes-shaped surface area.
- **macOS** — [apple/container](https://github.com/apple/container): each environment boots in its own lightweight VM in well under a second on Apple Silicon. Docker is used automatically as a fallback engine.

## Honest promise

- Faster/lighter than Docker for shells and small local services
- Reproducible across machines through a committed `lagoon.toml`
- Portable offline with `lagoon save` / `lagoon load`
- Good for normal dev work first, tiny AI tooling second
- Use Lagoon locally; export Docker when you need Docker ecosystems

## Best use cases

- Replace `docker compose up` for basic web projects on your Mac or laptop
- Spin up the same dev environment on an old laptop, Raspberry Pi, or mini PC
- Run a small local service stack without Docker Desktop overhead
- Package tiny AI tools like `llama.cpp`, `whisper.cpp`, or agent scripts
- Move a runtime to lab/offline/field machines with save/load

## Install

### macOS (Apple Silicon, macOS 15+)

```bash
brew install container && container system start   # apple/container engine
git clone https://github.com/imraghavojha/lagoon && cd lagoon
go build -o lagoon . && mv lagoon /usr/local/bin/  # or anywhere on PATH
```

No apple/container? Lagoon automatically falls back to Docker if it's installed.

### Linux (arm64 or amd64)

**Requirements:** bubblewrap, Nix, user namespaces enabled.

```bash
sudo apt install bubblewrap
sh <(curl -L https://nixos.org/nix/install) --no-daemon
source ~/.nix-profile/etc/profile.d/nix.sh

curl -fsSL https://raw.githubusercontent.com/imraghavojha/lagoon/main/install.sh | bash
```

## Core flow

```bash
lagoon init             # hardware-aware wizard: intent, preset, preview
lagoon shell            # main dev action: enter reproducible sandbox
lagoon run <cmd>        # one-off command in the sandbox
lagoon up               # run configured services with a live dashboard
lagoon ps               # status dashboard: machine, cache, processes
lagoon save runtime.nar # save runtime for offline machines
lagoon load runtime.nar # load runtime on another machine
lagoon docker image.tar # export a Docker image tar when needed
```

The same `lagoon.toml` works on both platforms: Linux resolves it through pinned Nix packages, macOS maps it to a small official container image (overridable with `image = "..."`).

### `lagoon init`

Detects RAM, architecture, core count, disk space, and (on macOS) the container engine. Lagoon uses this to suggest:

- Machine class: Pi-class, Laptop-class, Mini-PC, or Mac
- Default memory cap
- First-run warnings
- Which presets are safe

Detection guides defaults; it does not force behavior. The wizard asks for an intent (Dev Workspace, Service Stack, Portable Runtime), offers curated presets (Python, Node, Go, llama.cpp, whisper.cpp, Custom), and previews the final config before writing.

### `lagoon shell`

Enters the sandbox described by `lagoon.toml`. Warm starts feel instant — on macOS a fresh VM boots in about a second; on Linux the cached Nix environment skips resolution entirely.

Inside the sandbox:

- Your project is mounted at `/workspace`
- Only the configured environment is available
- Memory cap follows `memory_cap` unless overridden with `--memory`
- On Linux, `HOME` is ephemeral and network follows the config profile

### `lagoon up`

Starts every command in `[up]` and shows a live dashboard with services, status, logs, ports (with clickable `http://localhost:…` links), uptime, and engine/cache state. Services bind to real localhost ports — use your browser as you would with docker-compose.

```toml
[up]
app = "python3 -m http.server 8000"
```

Press `q` or `Ctrl+C` to stop services. On macOS each service runs in its own VM-isolated container, force-cleaned on exit.

### `lagoon ps`

Shows machine class, RAM/memory cap, arch and cores, cache warm/cold, configured and running services with ports and uptime, and the active project.

### `lagoon save` / `lagoon load`

Save an already-built environment, copy it to another machine, and load it without internet:

```bash
lagoon shell             # build/cache once on a connected machine
lagoon save runtime.nar  # Linux: Nix closure   macOS: OCI image tar
# copy the file
lagoon load runtime.nar
```

### `lagoon docker`

Exports a Docker-compatible image tar:

```bash
lagoon docker image.tar
docker load < image.tar
```

On Linux this builds via `nixpkgs.dockerTools` (no Docker daemon needed). On macOS it exports the environment's image through the engine.

## `lagoon.toml`

```toml
packages = ["python311", "uv"]
nixpkgs_commit = "26eaeac4e409d7b5a6bf6f90a2a2dc223c78d915"
nixpkgs_sha256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
profile = "network"
intent = "dev-workspace"
preset = "python"
memory_cap = "2g"
# image = "python:3.12-slim"   # optional: override the macOS container image

[up]
app = "python3 -m http.server 8000"
```

The Nix pin makes Linux environments bit-reproducible. On macOS the preset (or explicit `image`) picks the container image. Commit `lagoon.toml` so teammates and offline machines get the same runtime.

## Build from source

```bash
git clone https://github.com/imraghavojha/lagoon
cd lagoon
go build -o lagoon .
```
