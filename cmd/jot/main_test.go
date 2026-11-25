package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runJot runs the CLI in-process with a controlled environment and returns
// the exit code, stdout and stderr.
func runJot(t *testing.T, env map[string]string, args ...string) (int, string, string) {
	t.Helper()
	home := t.TempDir()
	base := map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"JOT_CONFIG": "", "LC_ALL": "", "LC_MESSAGES": "", "LANG": "",
	}
	for k, v := range env {
		base[k] = v
	}
	for k, v := range base {
		t.Setenv(k, v)
	}
	oldArgs, oldOut, oldErr := os.Args, os.Stdout, os.Stderr
	defer func() { os.Args, os.Stdout, os.Stderr = oldArgs, oldOut, oldErr }()
	or, ow, _ := os.Pipe()
	er, ew, _ := os.Pipe()
	os.Args = append([]string{"jot"}, args...)
	os.Stdout, os.Stderr = ow, ew
	outC, errC := make(chan string), make(chan string)
	go func() { b, _ := io.ReadAll(or); outC <- string(b) }()
	go func() { b, _ := io.ReadAll(er); errC <- string(b) }()
	code := run()
	ow.Close()
	ew.Close()
	return code, <-outC, <-errC
}

func writeCfg(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckConfigLanguage(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		cfg  string
		want []string
	}{
		{"no locale -> en default", nil, "", []string{"Config OK: (no file: defaults)", "language=en (default)"}},
		{"LANG=ja", map[string]string{"LANG": "ja_JP.UTF-8"}, "", []string{"設定OK: (ファイルなし: 既定値)", "language=ja (ロケール LANG=ja_JP.UTF-8)"}},
		{"LANG=C", map[string]string{"LANG": "C"}, "", []string{"Config OK:", "language=en (locale LANG=C)"}},
		{"LC_MESSAGES=C beats LANG=ja", map[string]string{"LC_MESSAGES": "C", "LANG": "ja_JP.UTF-8"}, "", []string{"Config OK:", "language=en (locale LC_MESSAGES=C)"}},
		{"LC_ALL=ja beats LANG=C", map[string]string{"LC_ALL": "ja_JP.UTF-8", "LANG": "C"}, "", []string{"設定OK:", "language=ja (ロケール LC_ALL=ja_JP.UTF-8)"}},
		{"config en beats LANG=ja", map[string]string{"LANG": "ja_JP.UTF-8"}, "language = \"en\"\n", []string{"Config OK:", "language=en (config file)"}},
		{"config ja beats LANG=C", map[string]string{"LANG": "C"}, "language = \"ja\"\n", []string{"設定OK:", "language=ja (設定ファイル)"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := []string{"--check-config"}
			if c.cfg != "" {
				args = append(args, "--config", writeCfg(t, c.cfg))
			}
			code, out, errOut := runJot(t, c.env, args...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errOut)
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in %q", w, out)
				}
			}
		})
	}
}

func TestInvalidLanguageExit2(t *testing.T) {
	for _, src := range []string{"language = \"jp\"\n", "language = 1\n", "language = \"JA\"\n", "language = \"\"\n"} {
		for _, env := range []map[string]string{nil, {"LANG": "ja_JP.UTF-8"}} {
			p := writeCfg(t, src)
			for _, extra := range [][]string{{"--check-config"}, {}} {
				// without --check-config Jot would start the TUI if the config
				// were valid; it must fail before that.
				code, out, errOut := runJot(t, env, append([]string{"--config", p}, extra...)...)
				if code != 2 || out != "" {
					t.Fatalf("%q %v %v: exit %d out %q err %q", src, env, extra, code, out, errOut)
				}
				if !strings.Contains(errOut, p) || !strings.Contains(errOut, "language") {
					t.Errorf("%q: stderr lacks file/reason: %q", src, errOut)
				}
				wantPrefix := "jot: config error: "
				if env != nil {
					wantPrefix = "jot: 設定エラー: "
				}
				if !strings.HasPrefix(errOut, wantPrefix) {
					t.Errorf("%q %v: %q", src, env, errOut)
				}
			}
		}
	}
	// also via JOT_CONFIG
	p := writeCfg(t, "language = \"jp\"\n")
	code, _, errOut := runJot(t, map[string]string{"JOT_CONFIG": p}, "--check-config")
	if code != 2 || !strings.Contains(errOut, `language must be "en" or "ja": "jp"`) {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestHelpLanguage(t *testing.T) {
	_, _, errOut := runJot(t, nil, "--help")
	if !strings.Contains(errOut, "Usage: jot [options]") || !strings.Contains(errOut, "validate the config file") {
		t.Fatal(errOut)
	}
	_, _, errOut = runJot(t, map[string]string{"LANG": "ja_JP.UTF-8"}, "--help")
	if !strings.Contains(errOut, "使い方: jot [オプション]") || !strings.Contains(errOut, "設定ファイルを検証して終了") {
		t.Fatal(errOut)
	}
	p := writeCfg(t, "language = \"ja\"\n")
	_, _, errOut = runJot(t, map[string]string{"LANG": "C"}, "--config="+p, "--help")
	if !strings.Contains(errOut, "使い方") {
		t.Fatal(errOut)
	}
}
