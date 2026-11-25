package i18n

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// Both catalogs have exactly the same key set.
func TestCatalogsHaveSameKeys(t *testing.T) {
	enKeys, jaKeys := Keys(EN), Keys(JA)
	if len(enKeys) < 100 {
		t.Fatalf("catalog unexpectedly small: %d", len(enKeys))
	}
	for _, k := range enKeys {
		if !Has(JA, k) {
			t.Errorf("key %q missing in ja", k)
		}
	}
	for _, k := range jaKeys {
		if !Has(EN, k) {
			t.Errorf("key %q missing in en", k)
		}
	}
}

var verbRe = regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)

// Each message has the same fmt verbs (in order) in both languages, and no
// message is empty.
func TestCatalogVerbsMatch(t *testing.T) {
	for _, k := range Keys(EN) {
		e, j := Raw(EN, k), Raw(JA, k)
		if strings.TrimSpace(e) == "" || strings.TrimSpace(j) == "" {
			t.Errorf("%s: empty message", k)
		}
		ev, jv := verbRe.FindAllString(e, -1), verbRe.FindAllString(j, -1)
		if strings.Join(ev, " ") != strings.Join(jv, " ") {
			t.Errorf("%s: verbs differ: en %v ja %v", k, ev, jv)
		}
	}
}

func isJapaneseRune(r rune) bool {
	return unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) || (r >= 0xFF01 && r <= 0xFF60) || (r >= 0x3000 && r <= 0x303F)
}

// English messages contain no Japanese characters.
func TestEnglishCatalogIsEnglish(t *testing.T) {
	for _, k := range Keys(EN) {
		for _, r := range Raw(EN, k) {
			if isJapaneseRune(r) {
				t.Errorf("%s: Japanese character %q in English message %q", k, r, Raw(EN, k))
				break
			}
		}
	}
}

// sourceFiles returns non-test Go sources of the app (not vendor, not the
// catalog itself).
func sourceFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"../../cmd", "../../internal"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") && !strings.HasSuffix(p, "catalog.go") && !strings.Contains(p, "testenv") {
				files = append(files, p)
			}
			return nil
		})
	}
	if len(files) < 10 {
		t.Fatalf("source scan found only %d files", len(files))
	}
	return files
}

var keyLitRe = regexp.MustCompile(`"((?:cli|cfg|action|ui|hint|in|st|err|tw|url)\.[a-z_]+)"`)

// Every message key referenced in the code exists in both catalogs, and every
// catalog key is referenced (action.* keys are built from config.Actions and
// checked in the ui tests).
func TestKeysUsedInCodeExist(t *testing.T) {
	used := map[string]bool{}
	for _, f := range sourceFiles(t) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range keyLitRe.FindAllStringSubmatch(string(b), -1) {
			used[m[1]] = true
			if !Has(EN, m[1]) || !Has(JA, m[1]) {
				t.Errorf("%s: key %q not in both catalogs", f, m[1])
			}
		}
	}
	var unused []string
	for _, k := range Keys(EN) {
		if !used[k] && !strings.HasPrefix(k, "action.") {
			unused = append(unused, k)
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Errorf("catalog keys never used: %v", unused)
	}
}

// Fixed UI strings live only in the catalog: no Japanese text remains in
// code outside comments (user data is never hard-coded either).
func TestNoHardcodedJapaneseInSource(t *testing.T) {
	for _, f := range sourceFiles(t) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "i18n:ignore") {
				continue
			}
			code := line
			if j := strings.Index(code, "//"); j >= 0 {
				code = code[:j]
			}
			for _, r := range code {
				if isJapaneseRune(r) {
					t.Errorf("%s:%d: hard-coded Japanese text: %s", f, i+1, strings.TrimSpace(line))
					break
				}
			}
		}
	}
}

func TestParseOnlyEnJa(t *testing.T) {
	for _, ok := range []string{"en", "ja"} {
		if l, good := Parse(ok); !good || string(l) != ok {
			t.Errorf("Parse(%q) rejected", ok)
		}
	}
	for _, bad := range []string{"jp", "JA", "EN", "ja_JP", "en_US", "english", "japanese", "", " ja"} {
		if _, good := Parse(bad); good {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}

func TestIsJapaneseLocale(t *testing.T) {
	for _, v := range []string{"ja_JP.UTF-8", "ja_JP.utf8", "ja_JP", "ja", "ja.UTF-8", "ja_JP.eucJP", "ja_JP@euro", "JA_JP.UTF-8"} {
		if !IsJapaneseLocale(v) {
			t.Errorf("%q should be Japanese", v)
		}
	}
	for _, v := range []string{"C", "POSIX", "C.UTF-8", "en_US.UTF-8", "en", "de_DE", "jap", "jav_ID", "", "fr_JA"} {
		if IsJapaneseLocale(v) {
			t.Errorf("%q should not be Japanese", v)
		}
	}
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// LC_ALL > LC_MESSAGES > LANG; the first NON-EMPTY one decides; English is
// the default and the fallback.
func TestFromLocalePriority(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want Lang
		from string
	}{
		{map[string]string{}, EN, ""},
		{map[string]string{"LANG": "ja_JP.UTF-8"}, JA, "LANG=ja_JP.UTF-8"},
		{map[string]string{"LANG": "en_US.UTF-8"}, EN, "LANG=en_US.UTF-8"},
		{map[string]string{"LANG": "C"}, EN, "LANG=C"},
		{map[string]string{"LANG": "POSIX"}, EN, "LANG=POSIX"},
		{map[string]string{"LANG": "ja"}, JA, "LANG=ja"},
		// the example from the order: LC_ALL empty, LC_MESSAGES=C, LANG=ja
		{map[string]string{"LC_ALL": "", "LC_MESSAGES": "C", "LANG": "ja_JP.UTF-8"}, EN, "LC_MESSAGES=C"},
		{map[string]string{"LC_MESSAGES": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"}, JA, "LC_MESSAGES=ja_JP.UTF-8"},
		{map[string]string{"LC_ALL": "ja_JP.UTF-8", "LC_MESSAGES": "C", "LANG": "C"}, JA, "LC_ALL=ja_JP.UTF-8"},
		{map[string]string{"LC_ALL": "C", "LC_MESSAGES": "ja_JP.UTF-8", "LANG": "ja_JP.UTF-8"}, EN, "LC_ALL=C"},
		{map[string]string{"LC_ALL": "", "LC_MESSAGES": "", "LANG": "ja_JP"}, JA, "LANG=ja_JP"},
		{map[string]string{"LANGUAGE": "ja"}, EN, ""}, // LANGUAGE is not consulted
	}
	for _, c := range cases {
		got, from := FromLocale(env(c.env))
		if got != c.want || from != c.from {
			t.Errorf("%v: got %s (%q), want %s (%q)", c.env, got, from, c.want, c.from)
		}
	}
}

func TestTranslateErrors(t *testing.T) {
	cause := errors.New("raw cause")
	e := Wrap(cause, "tw.task_not_found", "abcd1234")
	if !errors.Is(e, cause) {
		t.Fatal("Wrap must keep the cause")
	}
	if got := JA.Err(e); got != "タスク abcd1234 が見つかりません" {
		t.Fatal(got)
	}
	if got := EN.Err(e); got != "task abcd1234 not found" || e.Error() != got {
		t.Fatal(got)
	}
	// nested errors are translated too
	n := E("cfg.in_file", "/x/config.toml", E("cfg.bad_language", "jp"))
	if got := JA.Err(n); got != `/x/config.toml: language は "en" か "ja": "jp"` {
		t.Fatal(got)
	}
	// foreign errors (e.g. task output) are shown unchanged
	if got := JA.Err(errors.New("task: Could not find")); got != "task: Could not find" {
		t.Fatal(got)
	}
	if JA.T("no.such.key") != "no.such.key" {
		t.Fatal("unknown keys fall back to the key")
	}
}
