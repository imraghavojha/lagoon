package cmd

import (
	"fmt"
	"io/fs"
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

	// watch all subdirectories so changes in src/, tests/, etc. are caught
	filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		return watcher.Add(path)
	})

	fmt.Printf("%s watching %s\n\n", ok("→"), absPath)

	var (
		mu       sync.Mutex
		proc     *exec.Cmd
		procDone chan struct{} // closed when the current process exits
		debounce *time.Timer
	)

	launch := func() {
		fmt.Println(warn("!") + " starting: " + command)
		p, err := sandbox.Start(cfg, resolved, absPath, command, memFlag, envFlags)
		if err != nil {
			fmt.Fprintln(os.Stderr, fail("✗")+" "+err.Error())
			return
		}
		done := make(chan struct{})
		go func() { p.Wait(); close(done) }() // sole caller of Wait for this process
		mu.Lock()
		proc = p
		procDone = done
		mu.Unlock()
	}

	// stopCurrent signals the running process and waits for it to exit (500ms timeout).
	stopCurrent := func() {
		mu.Lock()
		p, done := proc, procDone
		proc = nil
		procDone = nil
		mu.Unlock()
		if p == nil || p.Process == nil {
			return
		}
		p.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			p.Process.Kill()
			<-done
		}
	}

	restart := func() {
		stopCurrent()
		fmt.Printf("\n%s file changed — restarting\n", warn("!"))
		launch()
	}

	launch()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigs:
			stopCurrent()
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
