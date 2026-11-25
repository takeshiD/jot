package ui

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/takeshiD/jot/internal/config"
	"github.com/takeshiD/jot/internal/i18n"
	"github.com/takeshiD/jot/internal/testenv"
	"github.com/takeshiD/jot/internal/tw"
)

func langConfig(l i18n.Lang) *config.Config {
	cfg := config.Default()
	cfg.Language, cfg.Lang = string(l), l
	return cfg
}

func hasJapanese(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) || (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF01 && r <= 0xFF60) {
			return true
		}
	}
	return false
}

// screens walks through every screen of the app and returns their texts.
// The data is ASCII so any Japanese on screen would be an untranslated UI
// string.
func screens(t *testing.T, m *Model) map[string]string {
	t.Helper()
	out := map[string]string{}
	snap := func(name string) { out[name] = plain(m) }
	snap("today-empty")
	key(m, "tab")
	snap("all")
	selectTitle(t, m, "Write report")
	key(m, "space")
	snap("status-added")
	key(m, "c")
	snap("status-showdone")
	key(m, "c", "v")
	snap("status-group")
	key(m, "v")
	key(m, "?")
	snap("help")
	key(m, "j", "j", "j", "j", "j", "j", "j", "j", "j", "j")
	snap("help-end")
	key(m, "esc")
	selectTitle(t, m, "Write report")
	key(m, "enter")
	snap("detail")
	key(m, "esc")
	key(m, "o")
	snap("urls")
	key(m, "2")
	snap("url-blocked")
	key(m, "esc")
	key(m, "a")
	snap("input-add")
	key(m, "esc")
	snap("status-cancel")
	key(m, "e")
	snap("input-title")
	key(m, "esc", "t")
	snap("input-tags")
	key(m, "esc", "p")
	snap("input-project")
	key(m, "esc", "n")
	snap("note")
	key(m, "esc")
	key(m, "D")
	snap("confirm")
	key(m, "n")
	snap("status-delcancel")
	key(m, "tab")
	snap("today")
	key(m, "x")
	snap("status-done")
	m.Update(tea.WindowSizeMsg{Width: 8, Height: 3})
	snap("tiny")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return out
}

func seedASCII(t *testing.T) {
	testenv.Setup(t)
	testenv.AddCLI(t, "project:work", "+report", "due:2030-01-02", "scheduled:2030-01-01", "Write report")
	u := testenv.AddCLI(t, "Buy milk")
	testenv.Task(t, u, "done")
	r := testenv.Export(t, "description:Write report")[0]["uuid"].(string)
	if err := testenv.Client().SetText(r, "jotnote", "see https://example.com/a and javascript:alert(1)"); err != nil {
		t.Fatal(err)
	}
}

// English: every fixed string on every screen is English.
func TestEnglishUIHasNoJapanese(t *testing.T) {
	seedASCII(t)
	m := newModel(t, langConfig(i18n.EN), &fakeClock{at("2026-11-24 09:00:00")}, &opened{})
	sc := screens(t, m)
	for name, s := range sc {
		if hasJapanese(s) {
			t.Errorf("%s: Japanese text in English UI:\n%s", name, s)
		}
	}
	want := map[string][]string{
		"today-empty":      {"Today 0", "All 1", "2026-11-24 Tue", "Today is empty", "? help  q quit  space Today  a add  x done"},
		"all":              {"▸ work (1)", "#report", "due 01/02"},
		"status-added":     {"Added to Today: Write report"},
		"status-showdone":  {"Showing completed tasks", "(no group)"},
		"status-group":     {"Grouping: none"},
		"help":             {"Jot keys", "Add to / remove from Today", "Move up within Today", "Config: none (defaults; e.g."},
		"help-end":         {"Show / hide completed", "Quit", "Input: enter confirm / esc cancel. Note: ctrl+s save / esc cancel"},
		"detail":           {"Status", "pending", "Group", "Tags", "Due", "Scheduled", "Note (jotnote)", "see https://example.com/a", "esc back"},
		"urls":             {"Choose a URL to open (http/https only)", "[blocked] javascript:alert(1) — javascript: is not allowed (http/https only)", "enter open  1-9 open by number  esc close"},
		"url-blocked":      {"Not opened (unsafe URL): javascript: is not allowed"},
		"input-add":        {"New task: ", "enter confirm  esc cancel"},
		"status-cancel":    {"Cancelled"},
		"input-title":      {"Title: Write report"},
		"input-tags":       {"Tags (space-separated): report"},
		"input-project":    {"Group (project): work"},
		"note":             {"Edit note: Write report", "ctrl+s save  esc cancel  enter newline"},
		"confirm":          {`Delete "Write report"? (y: delete / any other key: cancel)`, "y delete  n/esc keep"},
		"status-delcancel": {"Delete cancelled"},
		"today":            {" 1 ☐ Write report", "K up  J down"},
		"status-done":      {"Completed: Write report (x to undo)"},
		"tiny":             {"Jot: te"},
	}
	for name, ws := range want {
		for _, w := range ws {
			if !strings.Contains(sc[name], w) {
				t.Errorf("%s: missing %q in\n%s", name, w, sc[name])
			}
		}
	}
}

// Japanese: the same screens show the Japanese strings.
func TestJapaneseUI(t *testing.T) {
	seedASCII(t)
	m := newModel(t, langConfig(i18n.JA), &fakeClock{at("2026-11-24 09:00:00")}, &opened{})
	sc := screens(t, m)
	want := map[string][]string{
		"today-empty":      {"Today 0", "全タスク 1", "2026-11-24 (火)", "Todayは空です", "? ヘルプ  q 終了  space Today  a 追加  x 完了"},
		"all":              {"▸ work (1)", "#report", "期限01/02"},
		"status-added":     {"Todayに追加しました: Write report"},
		"status-showdone":  {"完了済みタスクを表示", "(グループなし)"},
		"status-group":     {"グループ表示: なし"},
		"help":             {"Jot キー一覧", "Todayに追加/解除", "Today内で上へ移動", "設定: なし (既定値, 例:"},
		"help-end":         {"完了済みの表示 切替", "終了", "入力欄: enter 確定 / esc 取消。メモ: ctrl+s 保存 / esc 取消"},
		"detail":           {"状態", "pending", "グループ", "タグ", "期限", "予定", "メモ (jotnote)", "esc 戻る"},
		"urls":             {"開くURLを選択 (http/https のみ開けます)", "[ブロック] javascript:alert(1) — javascript: は許可されていない", "enter 開く  1-9 番号で開く  esc 閉じる"},
		"url-blocked":      {"開きません (安全でないURL): javascript: は許可されていない"},
		"input-add":        {"新規タスク: ", "enter 確定  esc 取消"},
		"status-cancel":    {"取り消しました"},
		"input-title":      {"タイトル: Write report"},
		"input-tags":       {"タグ(空白区切り): report"},
		"input-project":    {"グループ(project): work"},
		"note":             {"メモ編集: Write report", "ctrl+s 保存  esc 取消  enter 改行"},
		"confirm":          {"「Write report」を削除しますか? (y: 削除 / その他: 取消)", "y 削除する  n/esc やめる"},
		"status-delcancel": {"削除を取り消しました"},
		"today":            {" 1 ☐ Write report", "K 上へ  J 下へ"},
		"status-done":      {"完了しました: Write report (x で取消)"},
	}
	for name, ws := range want {
		for _, w := range ws {
			if !strings.Contains(sc[name], w) {
				t.Errorf("%s: missing %q in\n%s", name, w, sc[name])
			}
		}
	}
}

// The help screen lists a description for every action in both languages.
func TestHelpListsAllActionsBothLanguages(t *testing.T) {
	for _, l := range []i18n.Lang{i18n.EN, i18n.JA} {
		m := New(Options{Config: langConfig(l), Backend: fakeBackend{}})
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
		key(m, "?")
		v := plain(m)
		for _, a := range config.Actions {
			d := l.T("action." + a.Name)
			if d == "action."+a.Name || !strings.Contains(v, d) {
				t.Errorf("%s: help lacks %s (%q)", l, a.Name, d)
			}
		}
	}
}

// User data is never translated and the stored Taskwarrior data does not
// depend on the language: the same operations in en and ja produce the same
// fields, and data written in one language is read unchanged by the other.
func TestDataIndependentOfLanguage(t *testing.T) {
	run := func(l i18n.Lang) map[string]any {
		testenv.Setup(t)
		m := newModel(t, langConfig(l), &fakeClock{at("2026-11-24 09:00:00")}, nil)
		key(m, "a") // in Today view: added to Today
		typeText(m, "日本語タイトル Report")
		key(m, "enter")
		key(m, "t")
		typeText(m, "タグ work")
		key(m, "enter")
		key(m, "p")
		typeText(m, "仕事")
		key(m, "enter")
		key(m, "n")
		typeText(m, "メモ https://example.com/日本")
		key(m, "ctrl+s")
		v := plain(m)
		for _, s := range []string{"日本語タイトル Report", "#タグ", "#work", "@仕事"} {
			if !strings.Contains(v, s) {
				t.Errorf("%s: user data %q not shown unchanged:\n%s", l, s, v)
			}
		}
		ts := testenv.Export(t)
		if len(ts) != 1 {
			t.Fatalf("%s: %v", l, ts)
		}
		got := ts[0]
		// a model in the OTHER language reads the same data
		other := i18n.JA
		if l == i18n.JA {
			other = i18n.EN
		}
		m2 := newModel(t, langConfig(other), &fakeClock{at("2026-11-24 09:00:00")}, nil)
		if tl := todayTitles(m2); len(tl) != 1 || tl[0] != "日本語タイトル Report" {
			t.Errorf("%s->%s: today list %v", l, other, tl)
		}
		return got
	}
	en, ja := run(i18n.EN), run(i18n.JA)
	for _, f := range []string{"description", "jotnote", "jottoday", "jotorder", "project", "status", "tags"} {
		if ansi.Strip(strings.TrimSpace(fmtAny(en[f]))) != ansi.Strip(strings.TrimSpace(fmtAny(ja[f]))) {
			t.Errorf("field %s differs: en=%v ja=%v", f, en[f], ja[f])
		}
	}
	if en["jottoday"] != "2026-11-24" || en["description"] != "日本語タイトル Report" || en["jotnote"] != "メモ https://example.com/日本" || en["project"] != "仕事" {
		t.Errorf("unexpected stored data: %v", en)
	}
}

func fmtAny(v any) string { return fmt.Sprint(v) }

// fakeBackend is an empty Taskwarrior for screens that need no data.
type fakeBackend struct{}

func (fakeBackend) Export() ([]tw.Task, error)            { return nil, nil }
func (fakeBackend) Add(string, ...string) (string, error) { return "", nil }
func (fakeBackend) Modify(string, ...string) error        { return nil }
func (fakeBackend) SetText(string, string, string) error  { return nil }
func (fakeBackend) Done(string) error                     { return nil }
func (fakeBackend) Undone(string) error                   { return nil }
func (fakeBackend) Delete(string) error                   { return nil }
