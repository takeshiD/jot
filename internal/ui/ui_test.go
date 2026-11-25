package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/takeshiD/jot/internal/config"
	"github.com/takeshiD/jot/internal/i18n"
	"github.com/takeshiD/jot/internal/testenv"
	"github.com/takeshiD/jot/internal/tw"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

type opened struct{ argv []string }

func newModel(t *testing.T, cfg *config.Config, clk *fakeClock, op *opened) *Model {
	t.Helper()
	if cfg == nil {
		cfg = config.Default()
	}
	// Existing UI tests assert the Japanese texts; the language is fixed
	// here so they never depend on the LANG of the test environment.
	if cfg.Language == "" {
		cfg.Lang = i18n.JA
	}
	o := Options{Config: cfg, Backend: testenv.Client(), Clock: clk.now, FakeTime: true}
	if op != nil {
		o.Opener = func(cmd []string, u string) ([]string, error) {
			op.argv = append(append([]string{}, cmd...), u)
			return op.argv, nil
		}
	}
	m := New(o)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

func key(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "ctrl+s":
			msg = tea.KeyMsg{Type: tea.KeyCtrlS}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m.Update(msg)
	}
}

func typeText(m *Model, s string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
}

func plain(m *Model) string { return ansi.Strip(m.View()) }

func todayTitles(m *Model) []string {
	var out []string
	for _, t := range m.todayList() {
		out = append(out, t.Description)
	}
	return out
}

func selectTitle(t *testing.T, m *Model, title string) {
	t.Helper()
	for i, r := range m.rows {
		if r.task != nil && r.task.Description == title {
			m.cursor = i
			m.syncSel()
			return
		}
	}
	t.Fatalf("no row %q", title)
}

// Today toggling, reorder with J/K, persistence across restart and across
// unrelated CLI changes; Today must not touch due/scheduled/status.
func TestTodayToggleOrderPersistence(t *testing.T) {
	testenv.Setup(t)
	for _, s := range []string{"A", "B", "C"} {
		testenv.AddCLI(t, s)
	}
	uD := testenv.AddCLI(t, "due:2030-01-02", "scheduled:2030-01-01", "D")
	clk := &fakeClock{at("2026-11-24 10:00:00")}
	m := newModel(t, nil, clk, nil)
	key(m, "tab") // -> All
	for _, s := range []string{"A", "B", "C", "D"} {
		selectTitle(t, m, s)
		key(m, "space")
	}
	if got := strings.Join(todayTitles(m), ""); got != "ABCD" {
		t.Fatal(got)
	}
	d := testenv.ByUUID(t, uD)
	if d["jottoday"] != "2026-11-24" || d["due"] != "20300102T000000Z" && d["due"] == nil || d["status"] != "pending" {
		t.Fatalf("D: %v", d)
	}
	key(m, "tab") // -> Today
	selectTitle(t, m, "C")
	key(m, "K", "K") // C to top
	selectTitle(t, m, "A")
	key(m, "J") // A below B
	if got := strings.Join(todayTitles(m), ""); got != "CBAD" {
		t.Fatal(got)
	}
	// CLI changes another task; restart Jot: order kept.
	testenv.Task(t, uD, "modify", "+other", "project:zz")
	testenv.AddCLI(t, "E")
	m2 := newModel(t, nil, clk, nil)
	if got := strings.Join(todayTitles(m2), ""); got != "CBAD" {
		t.Fatalf("after restart: %s", got)
	}
	if !strings.Contains(plain(m2), " 1 ☐ C") {
		t.Fatal(plain(m2))
	}
	// Order is stored as the jotorder UDA, visible to plain task export.
	for i, o := range testenv.Export(t, "jottoday.any:", "rc.uda.jottoday.type=string") {
		_ = i
		if o["jotorder"] == nil {
			t.Fatalf("missing jotorder: %v", o)
		}
	}
	// Remove from Today: UDAs cleared, due untouched.
	selectTitle(t, m2, "D")
	key(m2, "space")
	d = testenv.ByUUID(t, uD)
	if d["jottoday"] != nil || d["jotorder"] != nil || d["due"] == nil || d["scheduled"] == nil {
		t.Fatalf("D after removal: %v", d)
	}
	if strings.Join(todayTitles(m2), "") != "CBA" {
		t.Fatal(todayTitles(m2))
	}
}

// Date change: unfinished items carry over, completed ones leave Today the
// next day. The running UI notices the boundary on its tick.
func TestDateRollover(t *testing.T) {
	testenv.Setup(t)
	testenv.AddCLI(t, "持ち越す")
	testenv.AddCLI(t, "終わらせる")
	clk := &fakeClock{at("2026-11-24 23:59:58")}
	m := newModel(t, nil, clk, nil)
	key(m, "tab")
	selectTitle(t, m, "持ち越す")
	key(m, "space")
	selectTitle(t, m, "終わらせる")
	key(m, "space")
	key(m, "tab")
	selectTitle(t, m, "終わらせる")
	key(m, "x")
	if len(m.todayList()) != 2 || !strings.Contains(plain(m), "✓ 終わらせる") {
		t.Fatal(plain(m))
	}
	clk.t = at("2026-11-25 00:00:01")
	m.Update(tickMsg(clk.t))
	v := plain(m)
	if !strings.Contains(v, "日付が変わりました") || strings.Contains(v, "終わらせる") || !strings.Contains(v, "↻11/24") {
		t.Fatal(v)
	}
	// Re-adding a carried item? It is still on Today; toggling removes it.
	selectTitle(t, m, "持ち越す")
	key(m, "space")
	if len(m.todayList()) != 0 {
		t.Fatal(todayTitles(m))
	}
}

// Add / edit title / note / tags / project / done / undo / delete, all via keys.
func TestBasicOperationsByKeyboard(t *testing.T) {
	testenv.Setup(t)
	clk := &fakeClock{at("2026-11-24 09:00:00")}
	op := &opened{}
	m := newModel(t, nil, clk, op)
	key(m, "a")
	typeText(m, "買い物 +notag project:no")
	key(m, "enter")
	tasks := testenv.Export(t)
	if len(tasks) != 1 || tasks[0]["description"] != "買い物 +notag project:no" || tasks[0]["jottoday"] != "2026-11-24" {
		t.Fatalf("%v", tasks)
	}
	u := tasks[0]["uuid"].(string)
	key(m, "e")
	m.input.SetValue("")
	typeText(m, "牛乳を買う")
	key(m, "enter")
	key(m, "n")
	typeText(m, "メモ1行目 https://example.com/milk")
	key(m, "enter")
	typeText(m, "2行目 javascript:alert(1)")
	key(m, "ctrl+s")
	key(m, "t")
	typeText(m, " shop 日用品")
	key(m, "enter")
	key(m, "p")
	typeText(m, "home")
	key(m, "enter")
	got := testenv.ByUUID(t, u)
	if got["description"] != "牛乳を買う" || got["jotnote"] != "メモ1行目 https://example.com/milk\n2行目 javascript:alert(1)" || got["project"] != "home" {
		t.Fatalf("%v", got)
	}
	if tags := got["tags"].([]any); len(tags) != 2 {
		t.Fatalf("tags %v", tags)
	}
	// URL picker: first safe link is preselected; enter opens it.
	m.cfg.Browser.Command = []string{"mybrowser", "--flag"}
	key(m, "o")
	if !strings.Contains(plain(m), "[ブロック] javascript:alert(1") {
		t.Fatal(plain(m))
	}
	key(m, "2") // blocked one: refuses
	if op.argv != nil || !strings.Contains(plain(m), "開きません") {
		t.Fatal("blocked url must not open")
	}
	key(m, "enter")
	if strings.Join(op.argv, " ") != "mybrowser --flag https://example.com/milk" {
		t.Fatal(op.argv)
	}
	// tags: remove one
	key(m, "t")
	m.input.SetValue("shop")
	key(m, "enter")
	if tags := testenv.ByUUID(t, u)["tags"].([]any); len(tags) != 1 || tags[0] != "shop" {
		t.Fatal(tags)
	}
	// invalid tag -> error message, no crash
	key(m, "t")
	m.input.SetValue("bad:tag")
	key(m, "enter")
	if !strings.Contains(plain(m), "使えないタグ名") {
		t.Fatal(plain(m))
	}
	// done / undo
	key(m, "x")
	if testenv.ByUUID(t, u)["status"] != "completed" {
		t.Fatal("not completed")
	}
	key(m, "x")
	if testenv.ByUUID(t, u)["status"] != "pending" {
		t.Fatal("not reopened")
	}
	// delete with confirmation (n cancels, y deletes)
	key(m, "D", "n")
	if testenv.ByUUID(t, u)["status"] != "pending" {
		t.Fatal("deleted without confirmation")
	}
	key(m, "D", "y")
	if testenv.ByUUID(t, u)["status"] != "deleted" {
		t.Fatal("not deleted")
	}
	// help lists keys
	key(m, "?")
	if v := plain(m); !strings.Contains(v, "キー一覧") || !strings.Contains(v, "space") {
		t.Fatal(v)
	}
}

// Rendering: every line fits the terminal width, line count equals height,
// for all themes and narrow sizes, with long Japanese titles.
func TestLayoutNarrowAndThemes(t *testing.T) {
	testenv.Setup(t)
	long := strings.Repeat("とても長い日本語のタイトル", 6)
	testenv.AddCLI(t, "project:仕事", "+タグ1", "+tag2", long)
	testenv.AddCLI(t, "short")
	clk := &fakeClock{at("2026-11-24 09:00:00")}
	for _, combo := range []struct {
		theme string
		lang  i18n.Lang
	}{{"dark", i18n.JA}, {"light", i18n.JA}, {"dusk", i18n.JA}, {"dark", i18n.EN}, {"light", i18n.EN}, {"dusk", i18n.EN}} {
		theme := combo.theme
		cfg := config.Default()
		cfg.Theme = theme
		cfg.Language = string(combo.lang)
		cfg.Lang = combo.lang
		m := newModel(t, cfg, clk, nil)
		key(m, "tab")
		for _, sz := range [][2]int{{80, 24}, {60, 20}, {40, 12}, {120, 40}} {
			m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			for _, screen := range []func(){func() {}, func() { key(m, "?") }, func() { key(m, "esc", "enter") }, func() { key(m, "esc") }} {
				screen()
				lines := strings.Split(m.View(), "\n")
				if len(lines) != sz[1] {
					t.Fatalf("%s %v: %d lines", theme, sz, len(lines))
				}
				for _, l := range lines {
					if w := ansi.StringWidth(l); w > sz[0] {
						t.Fatalf("%s %v: line too wide (%d): %q", theme, sz, w, ansi.Strip(l))
					}
				}
				if !strings.Contains(ansi.Strip(lines[0]), "Jot") {
					t.Fatalf("header missing: %q", ansi.Strip(lines[0]))
				}
			}
		}
	}
}

// Themes produce different colors (dark vs light) for the selected row.
func TestThemesDiffer(t *testing.T) {
	if Themes["dark"].SelBG == Themes["light"].SelBG || Themes["dark"].FG == Themes["light"].FG {
		t.Fatal("dark and light must differ")
	}
	for _, n := range ThemeNames {
		if _, ok := Themes[n]; !ok {
			t.Fatal(n)
		}
	}
	if strings.Join(ThemeNames, ",") != strings.Join(config.ThemeNames, ",") {
		t.Fatal("theme name lists out of sync")
	}
}

// Custom keybindings from a config file drive the UI.
func TestCustomKeybindings(t *testing.T) {
	testenv.Setup(t)
	testenv.AddCLI(t, "one")
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte("[keys]\ntoggle_today = [\"T\"]\nswitch_view = [\"w\"]\n"), 0o644)
	cfg, err := config.Load(p, true)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t, cfg, &fakeClock{at("2026-11-24 09:00:00")}, nil)
	key(m, "w", "space")
	if len(m.todayList()) != 0 {
		t.Fatal("space should be unbound")
	}
	key(m, "T")
	if len(m.todayList()) != 1 {
		t.Fatal("T should toggle Today")
	}
	if !strings.Contains(plain(m), "T Today") {
		t.Fatal("hint should show the custom key:\n" + plain(m))
	}
}

// task failures are shown on screen; the app keeps running.
func TestTaskFailureShown(t *testing.T) {
	cfg := config.Default()
	cfg.Lang = i18n.JA
	m := New(Options{Config: cfg, Backend: tw.New(filepath.Join(t.TempDir(), "missing-task"))})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	key(m, "space", "a")
	typeText(m, "x")
	key(m, "enter")
	if v := plain(m); !strings.Contains(v, "エラー") || !strings.Contains(v, "見つかりません") {
		t.Fatal(v)
	}
}

// Several runes in one key message (fast typing / key repeat) are handled
// as separate key presses; detail view scrolls on small screens.
func TestMultiRuneKeysAndDetailScroll(t *testing.T) {
	testenv.Setup(t)
	for _, s := range []string{"A", "B", "C"} {
		testenv.AddCLI(t, s)
	}
	m := newModel(t, nil, &fakeClock{at("2026-11-24 09:00:00")}, nil)
	key(m, "tab")
	m.cfg.GroupBy = "none"
	key(m, "jj")
	if sel := m.selected(); sel == nil || sel.Description != "C" {
		t.Fatalf("selected %v", sel)
	}
	key(m, "n")
	typeText(m, strings.Repeat("メモの行\n", 30)+"最後の行")
	key(m, "ctrl+s")
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	key(m, "enter")
	if strings.Contains(plain(m), "最後の行") {
		t.Fatal("should not fit")
	}
	for i := 0; i < 40; i++ {
		key(m, "j")
	}
	if !strings.Contains(plain(m), "最後の行") {
		t.Fatal(plain(m))
	}
}

// Text arriving in one read (e.g. over SSH/tmux) is inserted literally in
// input and memo fields, even when it spells a key name ("home", "enter",
// "esc", "ctrl+s"...). Only real special keys act.
func TestTypedKeyNamesAreText(t *testing.T) {
	testenv.Setup(t)
	m := newModel(t, nil, &fakeClock{at("2026-11-24 09:00:00")}, nil)
	runes := func(s string) { m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}) }
	key(m, "a")
	for _, s := range []string{"home", "end", "enter", "esc", "up", "ctrl+c"} {
		runes(s)
	}
	if m.mode != modeInput || m.input.Value() != "homeendenterescupctrl+c" {
		t.Fatalf("mode=%v value=%q", m.mode, m.input.Value())
	}
	key(m, "enter")
	ts := testenv.Export(t)
	if len(ts) != 1 || ts[0]["description"] != "homeendenterescupctrl+c" {
		t.Fatalf("%v", ts)
	}
	key(m, "n")
	for _, s := range []string{"esc", "ctrl+s", "enter", "home"} {
		runes(s)
	}
	if m.mode != modeNote || m.note.Value() != "escctrl+senterhome" {
		t.Fatalf("mode=%v note=%q", m.mode, m.note.Value())
	}
	key(m, "ctrl+s")
	if got := testenv.Export(t)[0]["jotnote"]; got != "escctrl+senterhome" {
		t.Fatalf("note %v", got)
	}
	// a real Esc still cancels without saving
	key(m, "n")
	runes("draft")
	key(m, "esc")
	if m.mode == modeNote || testenv.Export(t)[0]["jotnote"] != "escctrl+senterhome" {
		t.Fatal("esc should cancel")
	}
}
