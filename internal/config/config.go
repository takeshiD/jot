// Package config loads Jot's TOML configuration and keybindings.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// Action names that keys can be bound to.
var Actions = []struct{ Name, Help string }{
	{"up", "上へ"},
	{"down", "下へ"},
	{"top", "先頭へ"},
	{"bottom", "末尾へ"},
	{"toggle_today", "Todayに追加/解除"},
	{"switch_view", "Today/全タスク 切替"},
	{"view_today", "Today画面"},
	{"view_all", "全タスク画面"},
	{"move_up", "Today内で上へ移動"},
	{"move_down", "Today内で下へ移動"},
	{"add", "タスク追加"},
	{"edit_title", "タイトル編集"},
	{"edit_note", "メモ編集(複数行)"},
	{"edit_tags", "タグ編集"},
	{"edit_project", "グループ(project)編集"},
	{"toggle_done", "完了/完了取消"},
	{"delete", "削除"},
	{"open_url", "メモ内URLを開く"},
	{"detail", "詳細表示"},
	{"toggle_group", "グループ表示 切替"},
	{"toggle_completed", "完了済みの表示 切替"},
	{"refresh", "再読込"},
	{"help", "ヘルプ"},
	{"quit", "終了"},
}

// Presets of keybindings.
var Presets = map[string]map[string][]string{
	"vim": {
		"up": {"k", "up"}, "down": {"j", "down"}, "top": {"g", "home"}, "bottom": {"G", "end"},
		"toggle_today": {"space"}, "switch_view": {"tab"}, "view_today": {"1"}, "view_all": {"2"},
		"move_up": {"K", "shift+up"}, "move_down": {"J", "shift+down"},
		"add": {"a"}, "edit_title": {"e"}, "edit_note": {"n"}, "edit_tags": {"t"}, "edit_project": {"p"},
		"toggle_done": {"x"}, "delete": {"D"}, "open_url": {"o"}, "detail": {"enter", "l"},
		"toggle_group": {"v"}, "toggle_completed": {"c"}, "refresh": {"r", "ctrl+r"},
		"help": {"?"}, "quit": {"q", "ctrl+c"},
	},
	"arrows": {
		"up": {"up", "ctrl+p"}, "down": {"down", "ctrl+n"}, "top": {"home"}, "bottom": {"end"},
		"toggle_today": {"space"}, "switch_view": {"tab"}, "view_today": {"1"}, "view_all": {"2"},
		"move_up": {"shift+up", "alt+up"}, "move_down": {"shift+down", "alt+down"},
		"add": {"a", "insert"}, "edit_title": {"e", "f2"}, "edit_note": {"n"}, "edit_tags": {"t"}, "edit_project": {"p"},
		"toggle_done": {"x"}, "delete": {"delete"}, "open_url": {"o"}, "detail": {"enter"},
		"toggle_group": {"v"}, "toggle_completed": {"c"}, "refresh": {"f5", "ctrl+r"},
		"help": {"?", "f1"}, "quit": {"q", "ctrl+c"},
	},
}

// Config is the parsed configuration.
type Config struct {
	Theme        string              `toml:"theme"`
	Preset       string              `toml:"preset"`
	DayStartHour int                 `toml:"day_start_hour"`
	CarryOver    *bool               `toml:"carry_over"`
	GroupBy      string              `toml:"group_by"`
	Keys         map[string][]string `toml:"keys"`
	Browser      struct {
		Command []string `toml:"command"`
	} `toml:"browser"`
	Taskwarrior struct {
		Command string `toml:"command"`
	} `toml:"taskwarrior"`

	Path     string              `toml:"-"` // file loaded ("" if none)
	Bindings map[string][]string `toml:"-"` // action -> keys (resolved)
	KeyMap   map[string]string   `toml:"-"` // key -> action (resolved)
}

// DefaultPath returns $XDG_CONFIG_HOME/jot/config.toml (or ~/.config/...).
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "jot", "config.toml")
}

// Resolve picks the config path: flag > $JOT_CONFIG > XDG default.
// explicit is true when the user named a file (it must then exist).
func Resolve(flag string) (path string, explicit bool) {
	if flag != "" {
		return flag, true
	}
	if e := os.Getenv("JOT_CONFIG"); e != "" {
		return e, true
	}
	return DefaultPath(), false
}

// Load reads and validates the config at path. A missing default file is
// not an error (defaults are used).
func Load(path string, explicit bool) (*Config, error) {
	c := &Config{}
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		md, err := toml.Decode(string(b), c)
		if err != nil {
			return nil, fmt.Errorf("%s: TOMLの構文エラー: %v", path, err)
		}
		if und := md.Undecoded(); len(und) > 0 {
			var ks []string
			for _, k := range und {
				ks = append(ks, k.String())
			}
			return nil, fmt.Errorf("%s: 不明な設定項目: %s", path, strings.Join(ks, ", "))
		}
		c.Path = path
	case errors.Is(err, os.ErrNotExist) && !explicit:
	default:
		return nil, fmt.Errorf("設定ファイルを読めません: %v", err)
	}
	if err := c.finish(); err != nil {
		if c.Path != "" {
			return nil, fmt.Errorf("%s: %v", c.Path, err)
		}
		return nil, err
	}
	return c, nil
}

// Default returns the built-in configuration.
func Default() *Config {
	c := &Config{}
	if err := c.finish(); err != nil {
		panic(err)
	}
	return c
}

// ThemeNames is set by the ui package (avoids an import cycle).
var ThemeNames = []string{"dark", "light", "dusk"}

func (c *Config) finish() error {
	if c.Theme == "" {
		c.Theme = "dark"
	}
	ok := false
	for _, n := range ThemeNames {
		ok = ok || n == c.Theme
	}
	if !ok {
		return fmt.Errorf("不明なテーマ %q (使えるもの: %s)", c.Theme, strings.Join(ThemeNames, ", "))
	}
	if c.Preset == "" {
		c.Preset = "vim"
	}
	base, ok := Presets[c.Preset]
	if !ok {
		return fmt.Errorf("不明なプリセット %q (使えるもの: vim, arrows)", c.Preset)
	}
	if c.DayStartHour < 0 || c.DayStartHour > 23 {
		return fmt.Errorf("day_start_hour は 0〜23: %d", c.DayStartHour)
	}
	if c.CarryOver == nil {
		t := true
		c.CarryOver = &t
	}
	switch c.GroupBy {
	case "":
		c.GroupBy = "project"
	case "project", "none":
	default:
		return fmt.Errorf("group_by は \"project\" か \"none\": %q", c.GroupBy)
	}
	if len(c.Browser.Command) > 0 && strings.TrimSpace(c.Browser.Command[0]) == "" {
		return errors.New("browser.command の先頭(実行ファイル)が空です")
	}
	valid := map[string]bool{}
	for _, a := range Actions {
		valid[a.Name] = true
	}
	c.Bindings = map[string][]string{}
	for k, v := range base {
		c.Bindings[k] = append([]string(nil), v...)
	}
	names := make([]string, 0, len(c.Keys))
	for n := range c.Keys {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, action := range names {
		if !valid[action] {
			return fmt.Errorf("[keys] 不明な操作名 %q", action)
		}
		keys := c.Keys[action]
		if len(keys) == 0 {
			return fmt.Errorf("[keys] %s: キーが空です", action)
		}
		for _, k := range keys {
			if err := ValidKey(k); err != nil {
				return fmt.Errorf("[keys] %s: %v", action, err)
			}
		}
		c.Bindings[action] = keys
	}
	c.KeyMap = map[string]string{}
	acts := make([]string, 0, len(c.Bindings))
	for a := range c.Bindings {
		acts = append(acts, a)
	}
	sort.Strings(acts)
	// User-overridden actions win over preset keys they collide with.
	for pass := 0; pass < 2; pass++ {
		for _, a := range acts {
			_, user := c.Keys[a]
			if (pass == 0) == user {
				continue
			}
			for _, k := range c.Bindings[a] {
				nk := NormalizeKey(k)
				if prev, dup := c.KeyMap[nk]; dup && prev != a {
					if user {
						if _, prevUser := c.Keys[prev]; prevUser {
							return fmt.Errorf("[keys] キー %q が %s と %s に重複しています", k, prev, a)
						}
						c.Bindings[prev] = remove(c.Bindings[prev], nk)
					}
				}
				c.KeyMap[nk] = a
			}
		}
	}
	for _, a := range Actions {
		if len(c.Bindings[a.Name]) == 0 && (a.Name == "quit" || a.Name == "help") {
			return fmt.Errorf("[keys] %s にキーが残っていません", a.Name)
		}
	}
	return nil
}

func remove(keys []string, nk string) []string {
	var out []string
	for _, k := range keys {
		if NormalizeKey(k) != nk {
			out = append(out, k)
		}
	}
	return out
}

var named = map[string]bool{
	"up": true, "down": true, "left": true, "right": true, "home": true, "end": true,
	"pgup": true, "pgdown": true, "enter": true, "tab": true, "shift+tab": true, "esc": true,
	"backspace": true, "delete": true, "insert": true, "space": true,
	"shift+up": true, "shift+down": true, "shift+left": true, "shift+right": true,
	"ctrl+up": true, "ctrl+down": true, "alt+up": true, "alt+down": true,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		named["ctrl+"+string(c)] = true
	}
	for i := 1; i <= 12; i++ {
		named[fmt.Sprintf("f%d", i)] = true
	}
}

// ValidKey checks a key name: a single printable character, alt+<char>, or a
// named key such as "enter", "ctrl+r", "shift+up", "f5".
func ValidKey(k string) error {
	if named[strings.ToLower(k)] {
		return nil
	}
	if utf8.RuneCountInString(k) == 1 && k != " " {
		return nil
	}
	if strings.HasPrefix(k, "alt+") && utf8.RuneCountInString(k) == 5 {
		return nil
	}
	return fmt.Errorf("不明なキー名 %q (例: \"j\", \"space\", \"ctrl+r\", \"shift+up\", \"f5\")", k)
}

// NormalizeKey maps config names to Bubble Tea key strings.
func NormalizeKey(k string) string {
	if utf8.RuneCountInString(k) == 1 {
		return k
	}
	l := strings.ToLower(k)
	switch l {
	case "space":
		return " "
	case "esc":
		return "esc"
	}
	return l
}

// KeysFor returns display names of keys bound to action.
func (c *Config) KeysFor(action string) []string { return c.Bindings[action] }

// CarryOverEnabled reports the carry_over setting.
func (c *Config) CarryOverEnabled() bool { return c.CarryOver == nil || *c.CarryOver }
