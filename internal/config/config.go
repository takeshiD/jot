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

	"github.com/tkcd/jot/internal/i18n"
)

// Action names that keys can be bound to (in help-screen order). Their
// descriptions are the i18n keys "action.<Name>".
var Actions = []struct{ Name string }{
	{"up"}, {"down"}, {"top"}, {"bottom"},
	{"toggle_today"}, {"switch_view"}, {"view_today"}, {"view_all"},
	{"move_up"}, {"move_down"},
	{"add"}, {"edit_title"}, {"edit_note"}, {"edit_tags"}, {"edit_project"},
	{"toggle_done"}, {"delete"}, {"open_url"}, {"detail"},
	{"toggle_group"}, {"toggle_completed"}, {"refresh"}, {"help"}, {"quit"},
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
	Language     string              `toml:"language"`
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
	Lang     i18n.Lang           `toml:"-"` // resolved display language
	LangFrom string              `toml:"-"` // "config", "locale" or "default"
	LangVar  string              `toml:"-"` // deciding locale variable, e.g. "LANG=ja_JP.UTF-8"
	langSet  bool                // `language` present in the file (even if "")
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

// Load reads and validates the config at path, taking the locale from the
// process environment. A missing default file is not an error (defaults are
// used). Errors are *i18n.Error values; render them with ErrorLang.
func Load(path string, explicit bool) (*Config, error) {
	return LoadEnv(path, explicit, os.Getenv)
}

// LoadEnv is Load with an explicit environment lookup (for tests).
func LoadEnv(path string, explicit bool, getenv func(string) string) (*Config, error) {
	c := &Config{}
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		md, err := toml.Decode(string(b), c)
		if err != nil {
			return nil, i18n.E("cfg.toml", path, err.Error())
		}
		if und := md.Undecoded(); len(und) > 0 {
			var ks []string
			for _, k := range und {
				ks = append(ks, k.String())
			}
			return nil, i18n.E("cfg.unknown_keys", path, strings.Join(ks, ", "))
		}
		c.Path = path
		c.langSet = md.IsDefined("language")
	case errors.Is(err, os.ErrNotExist) && !explicit:
	default:
		return nil, i18n.E("cfg.cannot_read", err.Error())
	}
	if err := c.finish(getenv); err != nil {
		if c.Path != "" {
			return nil, i18n.E("cfg.in_file", c.Path, err)
		}
		return nil, err
	}
	return c, nil
}

// Default returns the built-in configuration. Its language is English; it
// does not look at the environment.
func Default() *Config {
	c := &Config{}
	if err := c.finish(func(string) string { return "" }); err != nil {
		panic(err)
	}
	return c
}

// ResolveLang decides the display language: a valid `language` setting wins;
// otherwise the first non-empty of LC_ALL, LC_MESSAGES, LANG (Japanese
// locale -> ja, anything else -> en); otherwise en.
func ResolveLang(setting string, set bool, getenv func(string) string) (lang i18n.Lang, from, envVar string, err error) {
	if set || setting != "" {
		l, ok := i18n.Parse(setting)
		if !ok {
			return i18n.Default, "", "", i18n.E("cfg.bad_language", setting)
		}
		return l, "config", "", nil
	}
	l, v := i18n.FromLocale(getenv)
	if v == "" {
		return l, "default", "", nil
	}
	return l, "locale", v, nil
}

// ErrorLang returns the language used to report problems with the config
// file at path: its `language` value if that alone is valid, else the
// locale (so an invalid file still gets messages in a sensible language).
func ErrorLang(path string, getenv func(string) string) i18n.Lang {
	var probe struct {
		Language any `toml:"language"`
	}
	if b, err := os.ReadFile(path); err == nil {
		if _, err := toml.Decode(string(b), &probe); err == nil {
			if s, ok := probe.Language.(string); ok {
				if l, ok := i18n.Parse(s); ok {
					return l
				}
			}
		}
	}
	l, _ := i18n.FromLocale(getenv)
	return l
}

// ThemeNames is set by the ui package (avoids an import cycle).
var ThemeNames = []string{"dark", "light", "dusk"}

func (c *Config) finish(getenv func(string) string) error {
	lang, from, envVar, err := ResolveLang(c.Language, c.langSet, getenv)
	if err != nil {
		return err
	}
	c.Lang, c.LangFrom, c.LangVar = lang, from, envVar
	if c.Theme == "" {
		c.Theme = "dark"
	}
	ok := false
	for _, n := range ThemeNames {
		ok = ok || n == c.Theme
	}
	if !ok {
		return i18n.E("cfg.bad_theme", c.Theme, strings.Join(ThemeNames, ", "))
	}
	if c.Preset == "" {
		c.Preset = "vim"
	}
	base, ok := Presets[c.Preset]
	if !ok {
		return i18n.E("cfg.bad_preset", c.Preset)
	}
	if c.DayStartHour < 0 || c.DayStartHour > 23 {
		return i18n.E("cfg.bad_day_start", c.DayStartHour)
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
		return i18n.E("cfg.bad_group_by", c.GroupBy)
	}
	if len(c.Browser.Command) > 0 && strings.TrimSpace(c.Browser.Command[0]) == "" {
		return i18n.E("cfg.empty_browser")
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
			return i18n.E("cfg.unknown_action", action)
		}
		keys := c.Keys[action]
		if len(keys) == 0 {
			return i18n.E("cfg.empty_keys", action)
		}
		for _, k := range keys {
			if err := ValidKey(k); err != nil {
				return i18n.E("cfg.key_error", action, err)
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
							return i18n.E("cfg.dup_key", k, prev, a)
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
			return i18n.E("cfg.no_key_left", a.Name)
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
	return i18n.E("cfg.bad_key", k)
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
