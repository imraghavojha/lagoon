package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/preflight"
	"github.com/imraghavojha/lagoon/internal/sandbox"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "start all services defined in lagoon.toml [up] section",
	Long: `lagoon up

Starts every service in the [up] section of lagoon.toml inside the sandbox.
Services bind to real localhost ports — access them from your browser or
other tools on the host exactly as you would with docker-compose up.

  [up]
  web = "node server.js"
  api = "python3 -m flask run --port 8080"

Ctrl+C to stop all services.`,
	RunE: runUp,
}

// svcColors cycles through distinct terminal colors for service prefixes
var svcColors = []string{"12", "14", "10", "11", "13"}

func runUp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml — run 'lagoon init' first")
	}
	if len(cfg.Up) == 0 {
		return fmt.Errorf("no services defined in lagoon.toml\n\n  add an [up] section:\n\n  [up]\n  web = \"node server.js\"")
	}

	if err := preflight.RunAll(); err != nil {
		fmt.Fprintln(os.Stderr, fail("✗")+" "+err.Error())
		os.Exit(1)
	}

	absPath, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	cacheDir := projectCacheDir(absPath)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	sum, err := nix.GenerateShellNix(cfg, shellNixPath)
	if err != nil {
		return fmt.Errorf("generating shell.nix: %w", err)
	}

	resolved, hit := nix.LoadCache(cacheDir, sum)
	if hit {
		if _, err := os.Stat(resolved.BashPath); err != nil {
			hit = false
		}
	}

	if !hit {
		if runtime.GOARCH == "arm64" {
			fmt.Println(warn("!") + " arm: first run may take 10-60 min to compile packages")
		}
		env, err := resolveWithProgress(shellNixPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		resolved = env
		_ = nix.SaveCache(cacheDir, resolved, sum)
	}
	nix.CreateGCRoots(cacheDir, resolved)

	// services always need --share-net so ports bind to the host network
	netCfg := *cfg
	netCfg.Profile = "network"

	fmt.Println(warn("!") + " services run with network access enabled")

	// sort names for deterministic start order and color assignment
	names := make([]string, 0, len(cfg.Up))
	for n := range cfg.Up {
		names = append(names, n)
	}
	sort.Strings(names)

	type proc struct {
		cmd  *exec.Cmd
		done chan struct{}
		pw   *io.PipeWriter
	}
	procs := make([]proc, 0, len(names))

	for i, name := range names {
		color := svcColors[i%len(svcColors)]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
		prefix := style.Render(fmt.Sprintf("%-12s", name)+" |") + " "

		pr, pw := io.Pipe()
		c, err := sandbox.Build(&netCfg, resolved, absPath, cfg.Up[name], "", nil)
		if err != nil {
			// close already-open pipes before returning
			for _, p := range procs {
				p.pw.Close()
			}
			return fmt.Errorf("building sandbox for %q: %w", name, err)
		}
		c.Stdout = pw
		c.Stderr = pw
		if err := c.Start(); err != nil {
			pw.Close()
			for _, p := range procs {
				p.pw.Close()
			}
			return fmt.Errorf("starting %q: %w", name, err)
		}

		done := make(chan struct{})
		go func() { c.Wait(); close(done) }()

		// stream output with colored prefix
		go prefixLines(os.Stdout, prefix, pr)

		// print unexpected exits as warnings
		go func(n string, d chan struct{}) {
			<-d
			fmt.Fprintln(os.Stderr, warn("!")+" service "+n+" exited")
		}(name, done)

		procs = append(procs, proc{cmd: c, done: done, pw: pw})
		fmt.Println(ok("→")+" "+style.Render(name))
	}
	fmt.Println()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Println("\nstopping services…")
	var wg sync.WaitGroup
	for _, p := range procs {
		wg.Add(1)
		go func(p proc) {
			defer wg.Done()
			if p.cmd.Process == nil {
				return
			}
			p.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-p.done:
			case <-time.After(500 * time.Millisecond):
				p.cmd.Process.Kill()
				<-p.done
			}
			p.pw.Close()
		}(p)
	}
	wg.Wait()
	return nil
}

// prefixLines reads lines from src and writes each to dst with the given prefix.
func prefixLines(dst io.Writer, prefix string, src io.Reader) {
	s := bufio.NewScanner(src)
	for s.Scan() {
		fmt.Fprintln(dst, prefix+s.Text())
	}
}
