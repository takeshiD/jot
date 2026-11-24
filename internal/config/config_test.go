package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExampleConfigIsValid(t *testing.T) {
	c, err := Load("../../examples/config.toml", true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "dark" || c.Preset != "vim" || !c.CarryOverEnabled() {
		t.Fatalf("%+v", c)
	}
}

func TestDefaultsVim(t *testing.T) {
	c := Default()
	for k, want := range map[string]string{"j": "down", "k": "up", "down": "down", " ": "toggle_today", "J": "move_down", "K": "move_up", "?": "help", "q": "quit"} {
		if c.KeyMap[k] != want {
			t.Errorf("key %q -> %q, want %q", k, c.KeyMap[k], want)
		}
	}
}

func TestMissingDefaultFileIsOK(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.toml"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "nope.toml"), true); err == nil {
		t.Fatal("explicit missing file must be an error")
	}
}

func TestResolve(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x/cfg")
	t.Setenv("JOT_CONFIG", "")
	if p, e := Resolve(""); p != "/x/cfg/jot/config.toml" || e {
		t.Fatal(p)
	}
	t.Setenv("JOT_CONFIG", "/env.toml")
	if p, e := Resolve(""); p != "/env.toml" || !e {
		t.Fatal(p)
	}
	if p, _ := Resolve("/flag.toml"); p != "/flag.toml" {
		t.Fatal(p)
	}
}

func TestKeyOverride(t *testing.T) {
	c, err := Load(write(t, "[keys]\ntoggle_today = [\"T\"]\ndown = [\"n\", \"down\"]\n"), true)
	if err != nil {
		t.Fatal(err)
	}
	if c.KeyMap["T"] != "toggle_today" || c.KeyMap[" "] != "" {
		t.Fatalf("T=%q space=%q", c.KeyMap["T"], c.KeyMap[" "])
	}
	// "n" was edit_note in the preset; the user binding wins and edit_note loses it.
	if c.KeyMap["n"] != "down" {
		t.Fatal(c.KeyMap["n"])
	}
	for _, k := range c.Bindings["edit_note"] {
		if k == "n" {
			t.Fatal("n still on edit_note")
		}
	}
}

func TestArrowsPreset(t *testing.T) {
	c, err := Load(write(t, "preset = \"arrows\"\ntheme = \"light\"\n"), true)
	if err != nil {
		t.Fatal(err)
	}
	if c.KeyMap["j"] != "" || c.KeyMap["down"] != "down" || c.Theme != "light" {
		t.Fatal(c.KeyMap)
	}
}

func TestInvalidConfigs(t *testing.T) {
	cases := map[string]string{
		"theme = \"neon\"":                     "不明なテーマ",
		"preset = \"emacs\"":                   "不明なプリセット",
		"[keys]\nfly = [\"f\"]":                "不明な操作名",
		"[keys]\nup = [\"hyper+x\"]":           "不明なキー名",
		"[keys]\nup = []":                      "キーが空",
		"[keys]\nup = [\"z\"]\ndown = [\"z\"]": "重複",
		"colour = \"red\"":                     "不明な設定項目",
		"theme = ":                             "TOMLの構文エラー",
		"day_start_hour = 25":                  "day_start_hour",
		"group_by = \"tag\"":                   "group_by",
		"[browser]\ncommand = [\"\"]":          "browser.command",
	}
	for src, want := range cases {
		_, err := Load(write(t, src), true)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err=%v, want containing %q", src, err, want)
		}
	}
}
