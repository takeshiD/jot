package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tkcd/jot/internal/i18n"
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
		"theme = ":                             "TOMLの構文・型エラー",
		"day_start_hour = 25":                  "day_start_hour",
		"group_by = \"tag\"":                   "group_by",
		"[browser]\ncommand = [\"\"]":          "browser.command",
	}
	for src, want := range cases {
		_, err := Load(write(t, src), true)
		if err == nil || !strings.Contains(i18n.JA.Err(err), want) {
			t.Errorf("%q: err=%v, want containing %q", src, err, want)
		}
	}
}

func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

var (
	jaEnv = fakeEnv(map[string]string{"LANG": "ja_JP.UTF-8"})
	enEnv = fakeEnv(map[string]string{"LANG": "en_US.UTF-8"})
	noEnv = fakeEnv(nil)
)

// `language` accepts only "en" and "ja" and wins over the locale.
func TestLanguageSetting(t *testing.T) {
	for _, c := range []struct {
		src  string
		env  func(string) string
		want i18n.Lang
		from string
	}{
		{"language = \"en\"\n", jaEnv, i18n.EN, "config"},
		{"language = \"ja\"\n", enEnv, i18n.JA, "config"},
		{"language = \"ja\"\n", noEnv, i18n.JA, "config"},
		{"theme = \"dark\"\n", jaEnv, i18n.JA, "locale"},
		{"theme = \"dark\"\n", enEnv, i18n.EN, "locale"},
		{"theme = \"dark\"\n", noEnv, i18n.EN, "default"},
		{"theme = \"dark\"\n", fakeEnv(map[string]string{"LC_MESSAGES": "C", "LANG": "ja_JP.UTF-8"}), i18n.EN, "locale"},
		{"theme = \"dark\"\n", fakeEnv(map[string]string{"LC_ALL": "ja_JP.UTF-8", "LANG": "C"}), i18n.JA, "locale"},
	} {
		cfg, err := LoadEnv(write(t, c.src), true, c.env)
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if cfg.Lang != c.want || cfg.LangFrom != c.from {
			t.Errorf("%q: got %s/%s want %s/%s", c.src, cfg.Lang, cfg.LangFrom, c.want, c.from)
		}
	}
	// no file at all: locale decides
	cfg, err := LoadEnv(filepath.Join(t.TempDir(), "none.toml"), false, jaEnv)
	if err != nil || cfg.Lang != i18n.JA {
		t.Fatalf("%v %v", cfg, err)
	}
	if Default().Lang != i18n.EN {
		t.Fatal("Default() must be English and independent of the environment")
	}
}

// Invalid language values are validation errors naming the file.
func TestInvalidLanguage(t *testing.T) {
	for _, src := range []string{
		"language = \"jp\"\n",
		"language = \"JA\"\n",
		"language = \"ja_JP\"\n",
		"language = \"english\"\n",
		"language = \"\"\n",
		"language = 1\n",
		"language = true\n",
		"language = [\"ja\"]\n",
	} {
		p := write(t, src)
		_, err := LoadEnv(p, true, jaEnv)
		if err == nil {
			t.Errorf("%q: accepted", src)
			continue
		}
		for _, l := range []i18n.Lang{i18n.EN, i18n.JA} {
			msg := l.Err(err)
			if !strings.Contains(msg, p) || !strings.Contains(msg, "language") {
				t.Errorf("%q (%s): message lacks file or reason: %s", src, l, msg)
			}
		}
	}
	p := write(t, "language = \"jp\"\n")
	_, err := LoadEnv(p, true, noEnv)
	if got := i18n.EN.Err(err); got != p+`: language must be "en" or "ja": "jp"` {
		t.Fatal(got)
	}
	if got := i18n.JA.Err(err); got != p+`: language は "en" か "ja": "jp"` {
		t.Fatal(got)
	}
}

// Errors are reported in the file's language when that value is valid,
// otherwise in the locale's language.
func TestErrorLang(t *testing.T) {
	if l := ErrorLang(write(t, "language = \"ja\"\ntheme = \"x\"\n"), enEnv); l != i18n.JA {
		t.Fatal(l)
	}
	if l := ErrorLang(write(t, "language = \"jp\"\n"), jaEnv); l != i18n.JA {
		t.Fatal(l)
	}
	if l := ErrorLang(write(t, "language = \"jp\"\n"), noEnv); l != i18n.EN {
		t.Fatal(l)
	}
	if l := ErrorLang(write(t, "language = 1\n"), enEnv); l != i18n.EN {
		t.Fatal(l)
	}
}

// The bundled example keeps `language` commented out, so the locale decides.
func TestExampleConfigLeavesLanguageToLocale(t *testing.T) {
	b, err := os.ReadFile("../../examples/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "# language = ") {
		t.Fatal("example should show a commented-out language line")
	}
	for _, c := range []struct {
		env  func(string) string
		want i18n.Lang
	}{{jaEnv, i18n.JA}, {enEnv, i18n.EN}, {noEnv, i18n.EN}} {
		cfg, err := LoadEnv("../../examples/config.toml", true, c.env)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Lang != c.want || cfg.Language != "" {
			t.Errorf("got %s (%q), want %s", cfg.Lang, cfg.Language, c.want)
		}
	}
}

// Config errors are rendered in both languages.
func TestConfigErrorsTranslated(t *testing.T) {
	_, err := LoadEnv(write(t, "theme = \"neon\"\n"), true, noEnv)
	if en, ja := i18n.EN.Err(err), i18n.JA.Err(err); !strings.Contains(en, "unknown theme \"neon\"") || !strings.Contains(ja, "不明なテーマ \"neon\"") {
		t.Fatalf("%s / %s", en, ja)
	}
}
