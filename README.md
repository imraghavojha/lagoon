# Lagoon

Lagoon is a beautiful Linux CLI for reproducible dev environments and small local runtimes on low-end hardware.

It gives you the parts of Docker developers usually want locally — repeatable shells, small service stacks, offline portability, and an escape hatch to Docker — without a daemon or Kubernetes-shaped surface area.

## Honest promise

- Faster/lighter for shells and small local services
- Reproducible across machines through a pinned `lagoon.toml`
- Portable offline with `lagoon save` / `lagoon load`
- Good for normal dev work first, tiny AI tooling second
- Use Lagoon locally; export Docker when you need Docker ecosystems

## Best use cases

- Spin up the same dev environment on an old laptop, Raspberry Pi, or mini PC
- Run a small local service stack without Docker overhead
- Package tiny AI tools like `llama.cpp`, `whisper.cpp`, or agent scripts
- Move a runtime to lab/offline/field machines with save/load
- Export to Docker when a downstream system expects an image tar

## v1 scope

Lagoon v1 is intentionally narrow:

- Linux-only
- Single-machine
- Local runtime + portability tool
- Beautiful CLI around reproducible environments

Lagoon v1 is not cloud deploy, Kubernetes, a model download manager, multi-node orchestration, or perfect support for every distro edge case.

## Install

**Requirements:** Linux (arm64 or amd64), bubblewrap, Nix, user namespaces enabled.

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
lagoon up               # run configured services with a live dashboard
lagoon ps               # status dashboard: machine, cache, processes
lagoon save runtime.nar # save runtime for offline machines
lagoon load runtime.nar # load runtime on another machine
lagoon docker image.tar # export Docker image tar when needed
```

### `lagoon init`

Detects RAM, architecture, core count, and disk space. Lagoon uses this to suggest:

- Machine class: Pi-class, Laptop-class, or Mini-PC
- Default memory cap
- First-run warnings
- Which presets are safe
- Whether services are a comfortable default

Detection guides defaults; it does not force behavior.

The wizard asks for an intent:

- Dev Workspace
- Service Stack
- Portable Runtime

Then it offers curated presets:

- Python
- Node
- Go
- llama.cpp
- whisper.cpp
- Custom

Before writing, Lagoon previews the final config.

### `lagoon shell`

Enters the sandbox described by `lagoon.toml`. Warm starts skip Nix resolution and feel instant when the environment cache is valid.

Inside the sandbox:

- Your project is mounted at `/workspace`
- `HOME` is ephemeral
- Only configured packages are on `PATH`
- Network follows the config profile
- Memory cap follows `memory_cap` unless overridden with `--memory`

### `lagoon up`

Starts every command in `[up]` and shows a live dashboard with services, status, logs, memory, uptime, cache state, and networking. This is Lagoon's small-service-stack moment.

```toml
[up]
app = "python3 -m http.server 8000"
```

Press `q` or `Ctrl+C` to stop services.

### `lagoon ps`

Shows the current project status:

- Machine class
- RAM / memory cap
- Arch + cores
- Cache warm/cold
- Running services and shells
- Uptime and memory usage on Linux
- Network profile
- Cache size

### `lagoon save` / `lagoon load`

Save an already-built environment, copy it to another Linux machine, and load it without internet:

```bash
lagoon shell             # build/cache once on a connected machine
lagoon save runtime.nar
# copy runtime.nar
lagoon load runtime.nar
lagoon shell             # uses imported store paths offline
```

Redirection still works:

```bash
lagoon save > runtime.nar
```

### `lagoon docker`

Exports a Docker image tar without requiring the Docker daemon to build:

```bash
lagoon docker image.tar
docker load < image.tar
```

## `lagoon.toml`

```toml
packages = ["python311", "uv"]
nixpkgs_commit = "26eaeac4e409d7b5a6bf6f90a2a2dc223c78d915"
nixpkgs_sha256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
profile = "network"
intent = "dev-workspace"
preset = "python"
memory_cap = "2g"

[up]
app = "python3 -m http.server 8000"
```

The Nix pin makes environments reproducible. Commit `lagoon.toml` so teammates and offline machines get the same runtime.

## Build from source

```bash
git clone https://github.com/imraghavojha/lagoon
cd lagoon
go build -o lagoon .
```
