# envbox — Full Project Specification

A daemonless, reproducible sandboxed shell environment tool for Linux (ARM/Pi primary target).
No Docker. No root. No daemons. One config file. One command.

---

## What It Does

User runs `envbox init` once in a project directory. They type a list of nix package names.
envbox writes an `envbox.toml` with those packages and a pinned nixpkgs commit.
They commit that file. Anyone on any matching Linux machine runs `envbox shell` and gets
an identical sandboxed shell with exactly those tools available and nothing else.

---

## Target Platform

**Primary: ARM Linux (Raspberry Pi 4/5, Ubuntu 22.04+ or Raspberry Pi OS)**
**Secondary: x86-64 Linux**

All development and testing should happen in a VM before touching real hardware.
ARM binary cache gaps mean first-run builds may be long — this is expected and warned about.

---

## Tech Stack

```
Language:        Go 1.21+
CLI framework:   github.com/spf13/cobra
Output styling:  github.com/charmbracelet/lipgloss
Config parsing:  github.com/BurntSushi/toml
External tools:  nix-shell (subprocess), bwrap (subprocess via syscall.Exec)
```

No other Go libraries. Everything else is subprocess calls or stdlib.

---

## VM Testing Setup

Claude must set up a test VM before writing any functional code.
All features must be tested inside this VM, not on the host machine.

### Recommended VM setup

```bash
# On host machine — create ARM64 VM using QEMU or use multipass
multipass launch --name envbox-test --cpus 2 --memory 2G --disk 10G 22.04

# Or for closer Pi simulation:
# Use a Raspberry Pi OS arm64 image in QEMU

# SSH into VM
multipass shell envbox-test

# Inside VM — install prerequisites
sudo apt update
sudo apt install -y bubblewrap git curl

# Install Nix (single-user for simplicity in testing)
sh <(curl -L https://nixos.org/nix/install) --no-daemon
source ~/.nix-profile/etc/profile.d/nix.sh

# Verify both tools work
bwrap --version
nix-shell --version

# Install Go
wget https://go.dev/dl/go1.21.0.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-arm64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### VM testing workflow

Every checkpoint in this spec must be:
1. Built inside the VM (or cross-compiled and copied in)
2. Tested manually with the exact commands listed
3. All tests passing before moving to the next checkpoint
4. Git committed before moving on

Claude should use `scp` or `multipass transfer` to move the binary into the VM for testing.

---

## Known Problems and Their Solutions

These are all real problems that will break the tool silently or with confusing errors
if not handled. Every solution is implemented before moving on.

### Problem 1: nixpkgs is not pinned by default

**What breaks:** Two machines running `nix-shell` at different times get different package
versions. Reproducibility promise is completely broken.

**Solution:** The generated `shell.nix` always uses a pinned nixpkgs tarball with a
specific commit hash and sha256. These are stored in `envbox.toml` and committed to the
repo. envbox ships with a hardcoded default nixpkgs pin baked into the binary. User
never needs to find or update this unless they explicitly want to.

Default pin to use (nixpkgs-unstable, verified working):
```
commit: a3a793f1f9f2de4bb9e6d3cef1de219dba0a4af1
sha256: sha256:0000000000000000000000000000000000000000000000000000
```
NOTE: Claude must look up a real current nixpkgs-unstable commit and its sha256 before
using it. Run: `nix-prefetch-url --unpack https://github.com/NixOS/nixpkgs/archive/<COMMIT>.tar.gz`

### Problem 2: Empty rootfs has no bash or /usr/bin/env

**What breaks:** bwrap creates an empty root filesystem. There is no `/bin/bash`,
no `/usr/bin/env`, nothing. Every script with `#!/usr/bin/env python3` or `#!/bin/bash`
breaks immediately. This includes most of the programs in the nix store that users want.

**Solution:**
- Always inject `bash` and `coreutils` into every environment silently regardless of
  what the user specified. User never sees this, it just works.
- After resolving the nix environment, capture the bash path and env path from the
  nix store by running: `nix-shell shell.nix --run 'which bash && which env'`
- In the bwrap command, use `--symlink <nix-store-bash> /bin/sh` and
  `--symlink <nix-store-env> /usr/bin/env` to create these paths inside the sandbox.

### Problem 3: Host environment leaks into sandbox

**What breaks:** bwrap inherits the parent process's full environment. The user's host
`$PATH`, `$PYTHONPATH`, `$GEM_HOME`, random tool configs — all leak in. The sandbox
is not actually isolated.

**Solution:** Always pass `--clearenv` to bwrap, then explicitly re-set only:
```
PATH    = captured from nix-shell
HOME    = /home
TERM    = inherited from host (needed for terminal to work)
USER    = inherited from host (needed for tools that check username)
LANG    = C.UTF-8 (hardcoded, see Problem 4)
```
Everything else is stripped.

### Problem 4: Locale is missing, Python and others warn or fail

**What breaks:** No `/usr/lib/locale`, no locale configuration. Python immediately
throws `locale.Error: unsupported locale setting`. Perl throws similar warnings.
Some tools refuse to start.

**Solution:**
- Always set `LANG=C.UTF-8` via `--setenv` in bwrap.
- Use `--ro-bind-try /etc/localtime /etc/localtime` (the `-try` variant silently skips
  if the file doesn't exist on the host).

### Problem 5: /etc is empty, DNS and HTTPS break

**What breaks:** Without `/etc/resolv.conf`, any network call fails with DNS errors.
Without `/etc/ssl/certs`, HTTPS calls fail with certificate errors. Without
`/etc/passwd`, tools like `id` and `whoami` fail, and some programs refuse to start
because they can't resolve the current user.

**Solution:** Mount a minimal set of /etc files from the host. Use `--ro-bind-try`
for all of them so the sandbox doesn't fail on systems where a file might not exist:
```
--ro-bind-try /etc/resolv.conf   /etc/resolv.conf
--ro-bind-try /etc/ssl           /etc/ssl
--ro-bind-try /etc/ca-certificates /etc/ca-certificates
--ro-bind-try /etc/passwd        /etc/passwd
--ro-bind-try /etc/group         /etc/group
--ro-bind-try /etc/nsswitch.conf /etc/nsswitch.conf
--ro-bind-try /etc/localtime     /etc/localtime
```

### Problem 6: Unprivileged user namespaces may be disabled

**What breaks:** bwrap uses user namespaces to create the sandbox. On some hardened
kernels (common in enterprise or older Debian/Ubuntu), unprivileged user namespaces
are disabled. bwrap either fails silently or gives a cryptic kernel error.

**Solution:** Before doing anything, read `/proc/sys/kernel/unprivileged_userns_clone`.
If it exists and contains `0`, exit immediately with a clear human-readable message:
```
✗ User namespaces are disabled on this system.
  envbox requires unprivileged user namespaces to sandbox environments.
  To enable (requires root, ask your sysadmin):
    sudo sysctl -w kernel.unprivileged_userns_clone=1
  To make it permanent:
    echo 'kernel.unprivileged_userns_clone=1' | sudo tee /etc/sysctl.d/99-userns.conf
```

### Problem 7: First run is silent for 10+ minutes on ARM

**What breaks:** On ARM, many packages are not in the binary cache and must compile
from source. This can take 10-60 minutes. Without warning, the user thinks the tool
is frozen and kills it.

**Solution:**
- Before running nix-shell, detect architecture with `runtime.GOARCH`.
- If ARM, print a prominent warning:
```
! ARM detected: first run may take 10–60 minutes while packages compile.
  This only happens once. Subsequent runs use the cache.
  Do not interrupt this process.
```
- On all platforms, print before starting nix-shell:
```
  building environment... (first run downloads packages, may take a few minutes)
```

### Problem 8: nix-shell errors are unreadable

**What breaks:** If the user typos a package name, nix dumps a 40-line evaluation
error. The actual cause — "attribute 'pyhon311' missing" — is buried in the middle.

**Solution:**
- Capture both stdout and stderr from the nix-shell invocation.
- If nix-shell exits non-zero, scan stderr for the pattern `attribute '.*' missing`.
- If found, surface a clean error:
```
✗ Package not found: pyhon311
  Search for the correct name at: https://search.nixos.org/packages
  Then update your envbox.toml
```
- If not found, print the raw nix error below a separator so it's still accessible
  but clearly labeled as "raw nix output".

### Problem 9: HOME does not exist inside the sandbox

**What breaks:** HOME is set to `/home` but that directory doesn't exist in the empty
rootfs. Many tools (git, python, pip) try to write to $HOME and fail or crash.

**Solution:** Use bwrap's `--tmpfs /home` to create a writable in-memory home directory.
This means tool configs don't persist between sessions (intentional — the sandbox is
ephemeral) but tools don't crash.

### Problem 10: envbox.toml might not be committed to version control

**What breaks:** The whole reproducibility promise. If envbox.toml isn't committed,
teammates don't have it.

**Solution:** After `envbox init`, always print:
```
! Remember to commit envbox.toml to version control:
    git add envbox.toml && git commit -m "add envbox environment"
```

---

## File Structure

```
envbox/
  main.go
  cmd/
    root.go        — root command, prints help
    init.go        — envbox init (interactive setup)
    shell.go       — envbox shell (main command)
    clean.go       — envbox clean
  internal/
    config/
      config.go    — read/write envbox.toml, types
    nix/
      nix.go       — generate shell.nix, invoke nix-shell, parse errors
      template.go  — shell.nix template string
    sandbox/
      sandbox.go   — build bwrap args, syscall.Exec into sandbox
    preflight/
      check.go     — verify bwrap, nix-shell, user namespaces
  go.mod
  go.sum
```

---

## envbox.toml Format

```toml
packages = ["python311", "ffmpeg", "postgresql"]
nixpkgs_commit = "a3a793f1f9f2de4bb9e6d3cef1de219dba0a4af1"
nixpkgs_sha256 = "sha256:06jzngg5jm1f81sc4xfskvvgjy5bblz51xpl788mnps1wrkykfhp"
profile = "minimal"   # or "network"
```

`nixpkgs_commit` and `nixpkgs_sha256` are written automatically by `envbox init`
using the default pin bundled in the binary. User never writes these manually.

---

## Generated shell.nix Template

```nix
{ pkgs ? import (fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/{{COMMIT}}.tar.gz";
    sha256 = "{{SHA256}}";
  }) {}
}:

pkgs.mkShell {
  buildInputs = with pkgs; [
    bash
    coreutils
    {{PACKAGES}}
  ];
}
```

`bash` and `coreutils` are always injected. `{{PACKAGES}}` is the user's package list
from envbox.toml, one per line, indented.

---

## Commands

### `envbox init`

Checks if envbox.toml already exists — if so, asks if user wants to overwrite.

Interactive prompts:
```
envbox: no envbox.toml found.

Tip: search for package names at https://search.nixos.org/packages

What packages do you need? (space-separated)
> python311 ffmpeg

Use network access inside sandbox? (y/N)
> n

✓ Created envbox.toml
  packages: python311, ffmpeg
  nixpkgs: pinned to a3a793f (default)
  profile: minimal

! Remember to commit envbox.toml to version control:
    git add envbox.toml && git commit -m "add envbox environment"
```

### `envbox shell`

Full execution flow in order:

1. Run all preflight checks (see Preflight section)
2. Read envbox.toml — if missing, print: `✗ No envbox.toml found. Run 'envbox init' first.`
3. Detect architecture — if ARM, print ARM warning
4. Write shell.nix to `$XDG_CACHE_HOME/envbox/<project-hash>/shell.nix`
   (project hash = sha256 of absolute project path, first 8 chars)
5. Print: `  building environment... (first run may take several minutes)`
6. Run: `nix-shell <shell.nix path> --run 'which bash && which env && echo $PATH'`
   Capture stdout. On non-zero exit, parse and surface clean error (Problem 8 solution).
7. Parse stdout: extract bash path, env path, and PATH value
8. Construct bwrap command (see Sandbox section)
9. `syscall.Exec` into sandbox — this replaces the current process entirely

### `envbox clean`

Removes cached shell.nix and nix gcroots for the current project directory.
Prints what was removed. Does not touch the nix store itself (that's `nix-collect-garbage`).

---

## Preflight Checks

Run in this order. Exit immediately on first failure with a clear fix message.

```
1. Check bwrap on PATH
   Error: "✗ bubblewrap not found.\n  Install: sudo apt install bubblewrap"

2. Check nix-shell on PATH
   Error: "✗ nix not found.\n  Install: sh <(curl -L https://nixos.org/nix/install) --no-daemon\n  Then: source ~/.nix-profile/etc/profile.d/nix.sh"

3. Read /proc/sys/kernel/unprivileged_userns_clone
   If file exists and value == "0":
   Error: (full message from Problem 6 solution above)
   If file does not exist: skip check (means namespaces are enabled by default)
```

---

## Sandbox: bwrap Command Construction

Build this argument list in `sandbox/sandbox.go` and pass to `syscall.Exec`.

```
bwrap
  --ro-bind /nix/store /nix/store          # nix store read-only
  --bind <abs-project-path> /workspace      # project directory
  --tmpfs /tmp                              # writable temp
  --tmpfs /home                             # writable home (ephemeral)
  --dir /etc                                # create empty /etc dir
  --ro-bind-try /etc/resolv.conf /etc/resolv.conf
  --ro-bind-try /etc/ssl /etc/ssl
  --ro-bind-try /etc/ca-certificates /etc/ca-certificates
  --ro-bind-try /etc/passwd /etc/passwd
  --ro-bind-try /etc/group /etc/group
  --ro-bind-try /etc/nsswitch.conf /etc/nsswitch.conf
  --ro-bind-try /etc/localtime /etc/localtime
  --symlink <nix-bash-path> /bin/sh
  --symlink <nix-bash-path> /bin/bash
  --symlink <nix-env-path> /usr/bin/env
  --proc /proc
  --dev /dev
  --unshare-all
  --share-net                               # ONLY if profile = "network"
  --die-with-parent
  --clearenv
  --setenv HOME /home
  --setenv PATH <captured PATH from nix-shell>
  --setenv TERM <inherited from host>
  --setenv USER <inherited from host>
  --setenv LANG C.UTF-8
  --chdir /workspace
  -- <nix-bash-path>
```

Use `syscall.Exec` not `exec.Command`. This replaces the envbox process entirely so
the user is directly in the bash session. When they exit, they're back at their host shell
with no cleanup needed.

---

## Output Styling

Use lipgloss. Three styles only, no exceptions:

```go
var (
  success = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render  // green
  warning = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render  // yellow  
  failure = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render   // red
)

// Usage:
fmt.Println(success("✓") + " created envbox.toml")
fmt.Println(warning("!") + " ARM detected: first run may take a long time")
fmt.Println(failure("✗") + " package not found: pyhon311")
```

No spinners. No animations. No progress bars. Clean prefixed lines only.
The tool should feel like a well-written Unix tool, not an app.

---

## Build Checkpoints, Tests, and Git Commits

Claude must follow these checkpoints in order. Do not skip ahead.
At each checkpoint: build the binary, copy to VM, run every test listed,
fix anything that fails, then commit.

---

### Checkpoint 0: VM is ready

**Tests to run inside VM:**
```bash
bwrap --version          # must print version
nix-shell --version      # must print version
cat /proc/sys/kernel/unprivileged_userns_clone   # should print 1 or not exist
uname -m                 # should print aarch64 on Pi/ARM VM
```

All must pass. Fix VM setup if not.

**Do not proceed until all pass.**

---

### Checkpoint 1: Project skeleton compiles

Set up Go module, directory structure, all packages imported, cobra root command
that prints help.

**Build and test:**
```bash
# On host or in VM
go build -o envbox .
./envbox          # should print help text
./envbox --help   # should print help text
```

**Git commit:** `git commit -m "checkpoint 1: project skeleton compiles"`

---

### Checkpoint 2: Config read/write works

Implement `config/config.go` — the Config struct, ReadConfig, WriteConfig functions.
Implement `envbox init` command fully.

**Build and test inside VM:**
```bash
./envbox init
# Enter: python311 ffmpeg
# Enter: n for network

cat envbox.toml
# Must show packages, nixpkgs_commit, nixpkgs_sha256, profile

# Test overwrite prompt
./envbox init
# Should ask: envbox.toml already exists. Overwrite? (y/N)
# Enter: n
# Should exit without overwriting

# Test that envbox.toml has valid TOML
# Install toml validator or just check it parses:
./envbox shell 2>&1 | head -5
# Should say preflight check messages, not a parse error
```

**Git commit:** `git commit -m "checkpoint 2: init command and config read/write"`

---

### Checkpoint 3: Preflight checks work

Implement `preflight/check.go`. Run checks at the start of `envbox shell`.

**Build and test inside VM:**
```bash
# Test 1: everything installed — should pass silently
./envbox shell
# Should get past preflight (will fail later because no envbox.toml yet — that's fine)

# Test 2: bwrap not found
sudo mv /usr/bin/bwrap /usr/bin/bwrap.bak
./envbox shell
# Must print: ✗ bubblewrap not found. Install: sudo apt install bubblewrap
sudo mv /usr/bin/bwrap.bak /usr/bin/bwrap

# Test 3: nix not found
# Temporarily rename nix-shell
sudo mv $(which nix-shell) $(which nix-shell).bak
./envbox shell
# Must print: ✗ nix not found. Install from...
sudo mv $(which nix-shell).bak $(which nix-shell)

# Test 4: user namespaces disabled
echo 0 | sudo tee /proc/sys/kernel/unprivileged_userns_clone
./envbox shell
# Must print the full userns error message with sysctl fix instructions
echo 1 | sudo tee /proc/sys/kernel/unprivileged_userns_clone
```

**Git commit:** `git commit -m "checkpoint 3: preflight checks with clear error messages"`

---

### Checkpoint 4: shell.nix generation works

Implement `nix/template.go` and the shell.nix generation in `nix/nix.go`.

**Build and test inside VM:**
```bash
# Create a test envbox.toml manually
cat > envbox.toml << EOF
packages = ["cowsay"]
nixpkgs_commit = "<your real commit>"
nixpkgs_sha256 = "<your real sha256>"
profile = "minimal"
EOF

./envbox shell
# Should create shell.nix in cache dir and print building environment...

# Find and inspect the generated shell.nix
cat ~/.cache/envbox/*/shell.nix
# Must contain:
# - fetchTarball with the commit and sha256 from envbox.toml
# - bash in buildInputs
# - coreutils in buildInputs
# - cowsay in buildInputs
```

**Git commit:** `git commit -m "checkpoint 4: shell.nix generation with pinned nixpkgs"`

---

### Checkpoint 5: nix-shell invocation and error handling works

Implement running nix-shell, capturing PATH/bash/env paths, and error parsing.

**Build and test inside VM:**
```bash
# Test 1: valid package resolves
# (uses the cowsay envbox.toml from checkpoint 4)
./envbox shell
# Should print: building environment...
# Wait for nix to download (will be slow first time on ARM — expected)
# Should eventually print PATH output and proceed

# Test 2: invalid package name
cat > envbox.toml << EOF
packages = ["thisdoesnotexist12345"]
nixpkgs_commit = "<commit>"
nixpkgs_sha256 = "<sha256>"
profile = "minimal"
EOF

./envbox shell
# Must print: ✗ Package not found: thisdoesnotexist12345
# Must print: Search for the correct name at: https://search.nixos.org/packages
# Must NOT dump raw nix evaluation errors (or if it does, they must be clearly labeled)

# Test 3: verify PATH, bash path, env path are captured correctly
# Add temporary debug print to nix.go to print captured values, rebuild and test
# Remove debug print after verifying
```

**Git commit:** `git commit -m "checkpoint 5: nix-shell invocation and clean error handling"`

---

### Checkpoint 6: bwrap sandbox launches and is actually isolated

This is the most important checkpoint. Test isolation thoroughly.

**Build and test inside VM:**
```bash
# Use cowsay envbox.toml from earlier

./envbox shell
# Should drop you into a bash shell inside the sandbox

# Once inside the sandbox, run ALL of these:
echo "=== Testing isolation ==="

which cowsay          # must find cowsay from nix store
cowsay "it works"     # must print the cow

which python3 2>&1    # must say NOT found (we didn't ask for it)
which git 2>&1        # must say NOT found (we didn't ask for it)

echo $PATH            # must only contain nix store paths
echo $HOME            # must be /home
ls /home              # must be empty or just have basic dirs
ls /root 2>&1         # must fail — /root should not exist
ls /etc/shadow 2>&1   # must fail — sensitive files must not be visible

whoami                # must work (needs /etc/passwd)
curl --version 2>&1   # must say not found (no network tools in this env)

python3 -c "print('hi')" 2>&1   # must fail cleanly

echo "=== Testing /usr/bin/env ==="
/usr/bin/env bash --version   # must work

echo "=== Testing project dir ==="
ls /workspace         # must show your project files
pwd                   # must be /workspace

echo "=== Testing tmpfs home ==="
touch /home/testfile
ls /home/testfile     # must exist while in sandbox

exit

# After exiting sandbox, back on host:
ls /home/testfile 2>&1   # must NOT exist — tmpfs was ephemeral
```

**Test network profile:**
```bash
cat > envbox.toml << EOF
packages = ["curl"]
nixpkgs_commit = "<commit>"
nixpkgs_sha256 = "<sha256>"
profile = "network"
EOF

./envbox shell
curl https://example.com   # must succeed
exit
```

**Test minimal profile has no network:**
```bash
cat > envbox.toml << EOF
packages = ["curl"]
nixpkgs_commit = "<commit>"
nixpkgs_sha256 = "<sha256>"
profile = "minimal"
EOF

./envbox shell
curl https://example.com 2>&1   # must fail with network unreachable
exit
```

**Do not proceed until ALL isolation tests pass.**

**Git commit:** `git commit -m "checkpoint 6: sandbox isolation verified"`

---

### Checkpoint 7: ARM-specific behavior works

**Build and test inside ARM VM:**
```bash
uname -m   # verify aarch64

# Test ARM warning appears
./envbox shell
# Must print: ! ARM detected: first run may take 10-60 minutes...

# Test that packages actually compile and work on ARM
# (cowsay should be in binary cache for ARM — use it as the test case)
# If not in cache, this will compile from source — let it run
```

**Git commit:** `git commit -m "checkpoint 7: ARM warning and ARM environment works"`

---

### Checkpoint 8: Reproducibility verified

This verifies the whole point of the tool.

**Build and test:**
```bash
# In VM — create a project with envbox.toml
mkdir /tmp/test-repro
cd /tmp/test-repro
cat > envbox.toml << EOF
packages = ["python311"]
nixpkgs_commit = "<commit>"
nixpkgs_sha256 = "<sha256>"
profile = "minimal"
EOF

# First run — builds environment
envbox shell
python3 --version   # note the exact version printed
exit

# Delete nix cache for this project
envbox clean

# Second run — should produce identical environment
envbox shell
python3 --version   # must be IDENTICAL version to first run
exit

# If you have a second machine or second VM — copy envbox.toml there, run envbox shell
# python3 --version must be identical
```

**Git commit:** `git commit -m "checkpoint 8: reproducibility verified"`

---

### Checkpoint 9: envbox clean works

**Build and test:**
```bash
# After having run envbox shell at least once
ls ~/.cache/envbox/    # should show project cache dirs

envbox clean
# Must print what was removed

ls ~/.cache/envbox/    # cache for this project must be gone

# Run envbox shell again — should rebuild cleanly
./envbox shell
cowsay "rebuilt"
exit
```

**Git commit:** `git commit -m "checkpoint 9: clean command works"`

---

### Checkpoint 10: End-to-end user journey

Run the complete user journey from scratch on a clean VM snapshot
(or reset the VM to a fresh state).

```bash
# Fresh VM — only bwrap and nix installed, nothing else

git clone <your envbox repo>
cd envbox
go build -o envbox .

# Create a new project
mkdir /tmp/myproject
cd /tmp/myproject

# Run init
../envbox/envbox init
# Enter: python311
# Enter: n

cat envbox.toml   # verify contents

# Run shell
../envbox/envbox shell
# Wait for first run...
python3 --version   # must work
/usr/bin/env python3 --version   # must work
echo $HOME   # must be /home
ls /workspace   # must show project files
exit

# Verify reminder message was shown after init
# Verify ARM warning was shown (if on ARM)
# Verify no raw nix errors were dumped
```

**Git commit:** `git commit -m "checkpoint 10: full end-to-end user journey verified"`

---

## What NOT to Build

Do not build any of these in v0.1:

- TUI or interactive interface beyond the init prompts
- Flake support
- Auto-detection of project stack
- Windows or macOS support
- Custom bwrap flag exposure to users
- Environment export or sharing features
- Daemon or background service
- nixpkgs upgrade command (user edits envbox.toml manually if they want to change pin)
- GPU passthrough
- Multiple simultaneous environments

---

## Prompt to Give Claude Code

> Build a Go CLI tool called envbox. Use cobra for commands, lipgloss for output styling,
> and BurntSushi/toml for config parsing.
>
> Follow this exact file structure:
> envbox/main.go, cmd/root.go, cmd/init.go, cmd/shell.go, cmd/clean.go,
> internal/config/config.go, internal/nix/nix.go, internal/nix/template.go,
> internal/sandbox/sandbox.go, internal/preflight/check.go
>
> The envbox.toml format is:
> packages = ["python311", "ffmpeg"]
> nixpkgs_commit = "<commit>"
> nixpkgs_sha256 = "<sha256>"
> profile = "minimal"
>
> Before writing any code, set up a VM for testing using multipass with Ubuntu 22.04,
> install bubblewrap and nix inside it, and verify they work. All tests must run inside
> this VM.
>
> Then implement and test each checkpoint in the spec in order:
> 1. Skeleton compiles and prints help
> 2. init command writes envbox.toml correctly
> 3. Preflight checks with clear error messages for missing bwrap, missing nix, disabled userns
> 4. shell.nix generation with pinned nixpkgs, always including bash and coreutils
> 5. nix-shell invocation capturing PATH/bash/env paths, clean error for unknown packages
> 6. bwrap sandbox that is actually isolated: clearenv, tmpfs /home, /usr/bin/env symlink, ro-bind /nix/store, bind project to /workspace, minimal /etc mounts with --ro-bind-try
> 7. ARM detection warning
> 8. Reproducibility test
> 9. clean command
> 10. Full end-to-end journey on clean VM
>
> Use syscall.Exec (not exec.Command) to launch the sandbox so envbox replaces itself with bash.
> Use lipgloss with only three styles: green ✓ success, yellow ! warning, red ✗ error.
> No spinners, no animations.
> All error messages must be human readable with specific fix instructions.
>
> Git commit after each checkpoint passes all its tests. Do not proceed to the next
> checkpoint until all tests for the current one pass inside the VM.
