package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/preflight"
	"github.com/imraghavojha/lagoon/internal/sandbox"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch <command>",
	Short: "run a command in the sandbox, restart it when files change",
	Long: `lagoon watch "python3 server.py"

Starts the command inside the sandbox. Watches the project directory for
file changes and automatically restarts the command when anything changes.
300ms debounce prevents rapid restarts during saves. ctrl+c to exit.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWatch,
}

func runWatch(cmd *cobra.Command, args []string) error {
	command := strings.Join(args, " ")

	if err := preflight.RunAll(); err != nil {
		fmt.Fprintln(os.Stderr, fail("✗")+" "+err.Error())
		os.Exit(1)
	}

	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml — run 'lagoon init' first")
	}

	absPath, _ := filepath.Abs(".")
	cacheDir := projectCacheDir(absPath)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	sum, _ := nix.GenerateShellNix(cfg, shellNixPath)
	resolved, hit := nix.LoadCache(cacheDir, sum)
	if !hit {
		return fmt.Errorf("no cached environment — run 'lagoon shell' first to build it")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(absPath); err != nil {
		return fmt.Errorf("watching %s: %w", absPath, err)
	}

	fmt.Printf("%s watching %s\n\n", ok("→"), absPath)

	var (
		mu      sync.Mutex
		proc    *exec.Cmd
		debounce *time.Timer
	)

	launch := func() {
		fmt.Println(warn("!") + " starting: " + command)
		p, err := sandbox.Start(cfg, resolved, absPath, command, memFlag, envFlags)
		if err != nil {
			fmt.Fprintln(os.Stderr, fail("✗")+" "+err.Error())
			return
		}
		mu.Lock()
		proc = p
		mu.Unlock()
		go p.Wait() // reap the process when it exits
	}

	restart := func() {
		mu.Lock()
		p := proc
		mu.Unlock()
		if p != nil && p.Process != nil {
			p.Process.Signal(syscall.SIGTERM)
		}
		fmt.Printf("\n%s file changed — restarting\n", warn("!"))
		launch()
	}

	launch()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigs:
			mu.Lock()
			p := proc
			mu.Unlock()
			if p != nil && p.Process != nil {
				p.Process.Signal(syscall.SIGTERM)
				p.Wait()
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				mu.Lock()
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(300*time.Millisecond, restart)
				mu.Unlock()
			}

		case err := <-watcher.Errors:
			fmt.Fprintln(os.Stderr, warn("!")+" watch error: "+err.Error())
		}
	}
}
