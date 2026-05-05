package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/hardware"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/preflight"
	"github.com/imraghavojha/lagoon/internal/project"
	"github.com/imraghavojha/lagoon/internal/sandbox"
	"github.com/imraghavojha/lagoon/internal/ui"
	"github.com/spf13/cobra"
)

var upMemoryFlag string

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "start services with a live Lagoon dashboard",
	Long: `lagoon up

Starts every service in the [up] section of lagoon.toml inside the sandbox.
Services bind to real localhost ports — access them from your browser or
other tools on the host exactly as you would with docker-compose up.

  [up]
  web = "node server.js"
  api = "python3 -m flask run --port 8080"

Press q or Ctrl+C to stop all services.`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().StringVarP(&upMemoryFlag, "memory", "m", "", "limit each service via systemd-run (defaults to memory_cap when set)")
}

// svcColors cycles through distinct terminal colors for service prefixes.
var svcColors = []string{"12", "14", "10", "11", "13"}

func runUp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml — run 'lagoon init' first")
	}
	if len(cfg.Up) == 0 {
		fmt.Println(ui.Card("No services yet",
			ui.Bullet(ui.Hot.Render("!"), "lagoon up needs an [up] section in lagoon.toml"),
			ui.Bullet(ui.Dim.Render("•"), "example: [up] app = \"python3 -m http.server 8000\""),
		))
		return nil
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

	machine := hardware.Detect(absPath)
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

	netCfg := *cfg
	netCfg.Profile = "network"
	mem := upMemoryFlag
	if mem == "" {
		mem = cfg.MemoryCap
	}

	names := make([]string, 0, len(cfg.Up))
	for n := range cfg.Up {
		names = append(names, n)
	}
	sort.Strings(names)

	events := make(chan tea.Msg, 128)
	runners, err := startServices(&netCfg, resolved, absPath, names, mem, events)
	if err != nil {
		stopServices(runners)
		return err
	}
	defer stopServices(runners)
	for _, r := range runners {
		pid := 0
		if r.cmd.Process != nil {
			pid = r.cmd.Process.Pid
		}
		writeServicePIDFile(cacheDir, absPath, cfg, r.name, cfg.Up[r.name], pid)
	}

	status := project.Inspect(cfg, absPath, lagoonCacheBase())
	model := newUpModel(names, cfg.Up, runners, events, status, machine, mem, hit)
	_, err = tea.NewProgram(model).Run()
	return err
}

type runningService struct {
	name  string
	cmd   *exec.Cmd
	done  chan struct{}
	pipe  *io.PipeWriter
	start time.Time
}

type serviceLogMsg struct {
	Name string
	Line string
}

type serviceExitMsg struct {
	Name string
}

type upTickMsg time.Time

func startServices(cfg *config.Config, env *nix.ResolvedEnv, projectPath string, names []string, memory string, events chan<- tea.Msg) ([]*runningService, error) {
	runners := make([]*runningService, 0, len(names))
	for _, name := range names {
		pr, pw := io.Pipe()
		c, err := sandbox.Build(cfg, env, projectPath, cfg.Up[name], memory, nil)
		if err != nil {
			pw.Close()
			return runners, fmt.Errorf("building sandbox for %q: %w", name, err)
		}
		c.Stdout = pw
		c.Stderr = pw
		if err := c.Start(); err != nil {
			pw.Close()
			return runners, fmt.Errorf("starting %q: %w", name, err)
		}
		r := &runningService{name: name, cmd: c, done: make(chan struct{}), pipe: pw, start: time.Now()}
		runners = append(runners, r)
		go scanServiceLogs(name, pr, events)
		go func(r *runningService) {
			_ = r.cmd.Wait()
			close(r.done)
			events <- serviceExitMsg{Name: r.name}
		}(r)
	}
	return runners, nil
}

func scanServiceLogs(name string, src io.Reader, events chan<- tea.Msg) {
	s := bufio.NewScanner(src)
	for s.Scan() {
		events <- serviceLogMsg{Name: name, Line: s.Text()}
	}
}

func stopServices(runners []*runningService) {
	var wg sync.WaitGroup
	for _, r := range runners {
		wg.Add(1)
		go func(r *runningService) {
			defer wg.Done()
			if r == nil || r.cmd == nil || r.cmd.Process == nil {
				return
			}
			_ = r.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-r.done:
			case <-time.After(700 * time.Millisecond):
				_ = r.cmd.Process.Kill()
				<-r.done
			}
			_ = r.pipe.Close()
		}(r)
	}
	wg.Wait()
}

type serviceView struct {
	Name   string
	Cmd    string
	PID    int
	Status string
	Start  time.Time
	Logs   []string
	Color  string
	Ports  []string
}

type upModel struct {
	spinner spinner.Model
	events  <-chan tea.Msg
	status  project.Status
	machine hardware.Machine
	memory  string
	warm    bool
	start   time.Time
	order   []string
	svcs    map[string]*serviceView
}

func newUpModel(names []string, commands map[string]string, runners []*runningService, events <-chan tea.Msg, status project.Status, machine hardware.Machine, memory string, warm bool) upModel {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(ui.Accent)
	m := upModel{spinner: sp, events: events, status: status, machine: machine, memory: memory, warm: warm, start: time.Now(), order: names, svcs: map[string]*serviceView{}}
	for i, r := range runners {
		pid := 0
		if r.cmd.Process != nil {
			pid = r.cmd.Process.Pid
		}
		m.svcs[r.name] = &serviceView{Name: r.name, Cmd: commands[r.name], PID: pid, Status: "running", Start: r.start, Color: svcColors[i%len(svcColors)], Ports: inferPorts(commands[r.name])}
	}
	return m
}

func (m upModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitUpEvent(m.events), upTick())
}

func waitUpEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-events }
}

func upTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return upTickMsg(t) })
}

func (m upModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case serviceLogMsg:
		if svc := m.svcs[msg.Name]; svc != nil {
			svc.Logs = append(svc.Logs, msg.Line)
			if len(svc.Logs) > 8 {
				svc.Logs = svc.Logs[len(svc.Logs)-8:]
			}
		}
		return m, waitUpEvent(m.events)
	case serviceExitMsg:
		if svc := m.svcs[msg.Name]; svc != nil {
			svc.Status = "exited"
		}
		return m, waitUpEvent(m.events)
	case upTickMsg:
		return m, upTick()
	}
	return m, nil
}

func (m upModel) View() string {
	var b strings.Builder
	cache := "cold→warm"
	if m.warm {
		cache = "warm"
	}
	mem := m.memory
	if mem == "" {
		mem = "none"
	}
	fmt.Fprintf(&b, "%s\n\n", ui.Card("Lagoon up",
		ui.Chip("machine", string(m.machine.Class), ui.Accent)+"  "+ui.Chip("arch", m.machine.Arch, ui.Accent)+"  "+ui.Chip("cores", fmt.Sprint(m.machine.Cores), ui.Accent),
		ui.Chip("network", "on", ui.Good)+"  "+ui.Chip("memory cap", mem, ui.Warn)+"  "+ui.Chip("cache", cache, ui.Good),
		ui.Chip("uptime", time.Since(m.start).Round(time.Second).String(), ui.Accent)+"  "+ui.Chip("services", fmt.Sprint(len(m.order)), ui.Good),
	))
	b.WriteString(ui.Title.Render("Services") + "\n")
	for _, name := range m.order {
		svc := m.svcs[name]
		if svc == nil {
			continue
		}
		color := lipgloss.Color(svc.Color)
		nameStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
		statusStyle := ui.OK
		if svc.Status != "running" {
			statusStyle = ui.Err
		}
		mem := "?"
		if svc.PID > 0 && runtime.GOOS == "linux" {
			mem = readProcessMem(svc.PID)
		}
		fmt.Fprintf(&b, "  %s %s  pid:%d  ports:%s  mem:%s  up:%s\n", statusStyle.Render("●"), nameStyle.Render(svc.Name), svc.PID, portsLabel(svc.Ports), mem, time.Since(svc.Start).Round(time.Second))
		fmt.Fprintf(&b, "     %s\n", ui.Dim.Render(svc.Cmd))
	}
	b.WriteString("\n" + ui.Title.Render("Logs") + "\n")
	for _, name := range m.order {
		svc := m.svcs[name]
		if svc == nil || len(svc.Logs) == 0 {
			continue
		}
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(svc.Color)).Bold(true)
		for _, line := range svc.Logs {
			if len(line) > 100 {
				line = line[:100] + "…"
			}
			fmt.Fprintf(&b, "  %s │ %s\n", nameStyle.Render(fmt.Sprintf("%-10s", svc.Name)), line)
		}
	}
	if len(m.order) == 0 {
		b.WriteString(ui.Dim.Render("  no services configured") + "\n")
	}
	b.WriteString("\n" + ui.Dim.Render(m.spinner.View()+" q/ctrl+c stops services") + "\n")
	return b.String()
}

// prefixLines reads lines from src and writes each to dst with the given prefix.
func prefixLines(dst io.Writer, prefix string, src io.Reader) {
	s := bufio.NewScanner(src)
	for s.Scan() {
		fmt.Fprintln(dst, prefix+s.Text())
	}
}
