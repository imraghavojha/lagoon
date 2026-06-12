package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/imraghavojha/lagoon/internal/engine"
)

// RunAll verifies the platform runtime is usable.
// macOS: a container engine (apple/container or docker) must be installed and running.
// Linux: bwrap, nix-shell, and user namespaces must be available.
// stops at the first failure — no point continuing if the runtime is missing.
func RunAll() error {
	if engine.UseContainers() {
		eng, err := engine.Detect()
		if err != nil {
			return err
		}
		return eng.EnsureRunning()
	}
	if err := checkBwrap(); err != nil {
		return err
	}
	if err := checkNix(); err != nil {
		return err
	}
	return checkUserns()
}

func checkBwrap() error {
	if _, err := exec.LookPath("bwrap"); err != nil {
		return fmt.Errorf("bubblewrap not found.\n  install: sudo apt install bubblewrap")
	}
	return nil
}

func checkNix() error {
	if _, err := exec.LookPath("nix-shell"); err != nil {
		return fmt.Errorf("nix not found.\n  install: sh <(curl -L https://nixos.org/nix/install) --no-daemon\n  then: source ~/.nix-profile/etc/profile.d/nix.sh")
	}
	return nil
}

// checkUserns reads the kernel flag for unprivileged user namespaces.
// if the file doesn't exist, namespaces are enabled by default — that's fine.
func checkUserns() error {
	data, err := os.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
	if err != nil {
		// file doesn't exist means the kernel allows it by default
		return nil
	}

	if strings.TrimSpace(string(data)) == "0" {
		return fmt.Errorf(`user namespaces are disabled on this system.
  lagoon requires unprivileged user namespaces to sandbox environments.
  to enable (requires root, ask your sysadmin):
    sudo sysctl -w kernel.unprivileged_userns_clone=1
  to make it permanent:
    echo 'kernel.unprivileged_userns_clone=1' | sudo tee /etc/sysctl.d/99-userns.conf`)
	}

	return nil
}
