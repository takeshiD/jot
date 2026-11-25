// Command jot is a Today-centric keyboard TUI for Taskwarrior.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"github.com/takeshiD/jot/internal/config"
	"github.com/takeshiD/jot/internal/i18n"
	"github.com/takeshiD/jot/internal/tw"
	"github.com/takeshiD/jot/internal/ui"
)

var version = "0.1.0"

func main() { os.Exit(run()) }

// parseNow accepts RFC3339, "2006-01-02T15:04:05" / "2006-01-02 15:04" (local) or "2006-01-02".
func parseNow(s string) (time.Time, error) {
	for _, l := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		var t time.Time
		var err error
		if l == time.RFC3339 {
			t, err = time.Parse(l, s)
		} else {
			t, err = time.ParseInLocation(l, s, time.Local)
		}
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, i18n.E("cli.bad_now", s)
}

// preConfigFlag finds the value of -config/--config before flags are parsed,
// so that --help and flag errors use the configured language.
func preConfigFlag(args []string) string {
	for i, a := range args {
		for _, p := range []string{"--config", "-config"} {
			if a == p && i+1 < len(args) {
				return args[i+1]
			}
			if strings.HasPrefix(a, p+"=") {
				return a[len(p)+1:]
			}
		}
	}
	return ""
}

func run() int {
	prePath, _ := config.Resolve(preConfigFlag(os.Args[1:]))
	// Language for messages before (or instead of) a valid config: the
	// file's `language` if valid, else the locale, else English.
	tr := config.ErrorLang(prePath, os.Getenv)
	fs := flag.NewFlagSet("jot", flag.ContinueOnError)
	cfgPath := fs.String("config", "", tr.T("cli.flag_config"))
	themeFlag := fs.String("theme", "", tr.T("cli.flag_theme"))
	nowFlag := fs.String("now", "", tr.T("cli.flag_now"))
	showVersion := fs.Bool("version", false, tr.T("cli.flag_version"))
	check := fs.Bool("check-config", false, tr.T("cli.flag_check"))
	printRC := fs.Bool("print-taskrc", false, tr.T("cli.flag_taskrc"))
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), tr.T("cli.usage_head", version))
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), tr.T("cli.usage_env"))
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Println("jot", version)
		return 0
	}
	if *printRC {
		fmt.Print(tw.TaskrcSnippet())
		return 0
	}
	path, explicit := config.Resolve(*cfgPath)
	cfg, err := config.Load(path, explicit)
	if err != nil {
		tr = config.ErrorLang(path, os.Getenv)
		fmt.Fprintln(os.Stderr, tr.T("cli.config_error", err))
		return 2
	}
	tr = cfg.Lang
	if *themeFlag != "" {
		if _, ok := ui.Themes[*themeFlag]; !ok {
			fmt.Fprintln(os.Stderr, tr.T("cli.unknown_theme", *themeFlag))
			return 2
		}
		cfg.Theme = *themeFlag
	}
	if *check {
		src := cfg.Path
		if src == "" {
			src = tr.T("cli.no_file")
		}
		lang := tr.T("cli.lang_default", cfg.Lang)
		switch cfg.LangFrom {
		case "config":
			lang = tr.T("cli.lang_config", cfg.Lang)
		case "locale":
			lang = tr.T("cli.lang_locale", cfg.Lang, cfg.LangVar)
		}
		fmt.Println(tr.T("cli.check_ok", src, cfg.Theme, cfg.Preset, lang))
		return 0
	}
	clock := time.Now
	fake := false
	nowStr := *nowFlag
	if nowStr == "" {
		nowStr = os.Getenv("JOT_NOW")
	}
	if nowStr != "" {
		base, err := parseNow(nowStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "jot:", tr.Err(err))
			return 2
		}
		start := time.Now()
		clock = func() time.Time { return base.Add(time.Since(start)) }
		fake = true
	}
	bin := cfg.Taskwarrior.Command
	if e := os.Getenv("JOT_TASK"); e != "" {
		bin = e
	}
	m := ui.New(ui.Options{Config: cfg, Backend: tw.New(bin), Clock: clock, FakeTime: fake})
	// Synchronous renderer: each key press is drawn immediately (see ui.Immediate).
	r := ui.NewImmediate(m, os.Stdout)
	// With tea.WithoutRenderer Bubble Tea does not switch the tty to raw
	// mode, so do it here (and always restore it).
	if term.IsTerminal(os.Stdin.Fd()) {
		st, err := term.MakeRaw(os.Stdin.Fd())
		if err != nil {
			fmt.Fprintln(os.Stderr, "jot:", err)
			return 1
		}
		defer term.Restore(os.Stdin.Fd(), st)
	}
	r.Enter()
	size := func() (tea.WindowSizeMsg, bool) {
		w, h, err := term.GetSize(os.Stdout.Fd())
		return tea.WindowSizeMsg{Width: w, Height: h}, err == nil && w > 0 && h > 0
	}
	if s, ok := size(); ok {
		m.Update(s)
	}
	p := tea.NewProgram(r, tea.WithoutRenderer())
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			if s, ok := size(); ok {
				p.Send(s)
			}
		}
	}()
	_, err = p.Run()
	r.Leave()
	if err != nil {
		fmt.Fprintln(os.Stderr, "jot:", err)
		return 1
	}
	return 0
}
