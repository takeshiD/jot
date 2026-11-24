// Command jot is a Today-centric keyboard TUI for Taskwarrior.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"github.com/tkcd/jot/internal/config"
	"github.com/tkcd/jot/internal/tw"
	"github.com/tkcd/jot/internal/ui"
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
	return time.Time{}, fmt.Errorf("時刻を解釈できません: %q (例: 2026-11-24T23:59:50)", s)
}

func run() int {
	fs := flag.NewFlagSet("jot", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "設定ファイル (既定: $JOT_CONFIG または $XDG_CONFIG_HOME/jot/config.toml)")
	themeFlag := fs.String("theme", "", "テーマを上書き (dark, light, dusk)")
	nowFlag := fs.String("now", "", "現在時刻を差し替える (テスト用; $JOT_NOW と同じ)")
	showVersion := fs.Bool("version", false, "バージョンを表示")
	check := fs.Bool("check-config", false, "設定ファイルを検証して終了")
	printRC := fs.Bool("print-taskrc", false, "taskrc に追記できるUDA定義を表示 (任意)")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Jot %s — Today中心のTaskwarrior TUI\n\n使い方: jot [オプション]\n\n", version)
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "\n環境変数: JOT_CONFIG, JOT_NOW, JOT_TASK (taskコマンドのパス), TASKRC, TASKDATA")
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
		fmt.Fprintln(os.Stderr, "jot: 設定エラー:", err)
		return 2
	}
	if *themeFlag != "" {
		if _, ok := ui.Themes[*themeFlag]; !ok {
			fmt.Fprintf(os.Stderr, "jot: 不明なテーマ %q (dark, light, dusk)\n", *themeFlag)
			return 2
		}
		cfg.Theme = *themeFlag
	}
	if *check {
		src := cfg.Path
		if src == "" {
			src = "(ファイルなし: 既定値)"
		}
		fmt.Printf("設定OK: %s (theme=%s, preset=%s)\n", src, cfg.Theme, cfg.Preset)
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
			fmt.Fprintln(os.Stderr, "jot:", err)
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
