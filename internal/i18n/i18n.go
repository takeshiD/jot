// Package i18n holds Jot's UI message catalog (English and Japanese) and the
// rules that choose the display language.
//
// Only Jot's own fixed strings are translated. User data (titles, notes,
// tags, project names, URLs), raw Taskwarrior values and the output of the
// task command are never passed through the catalog.
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// Lang is a supported display language.
type Lang string

// Supported languages. English is the default and the fallback.
const (
	EN Lang = "en"
	JA Lang = "ja"
)

// Default is used when nothing selects a language.
const Default = EN

// Supported lists the accepted values of the `language` setting.
var Supported = []Lang{EN, JA}

// Parse accepts exactly "en" or "ja" (no aliases such as "jp").
func Parse(s string) (Lang, bool) {
	switch Lang(s) {
	case EN, JA:
		return Lang(s), true
	}
	return "", false
}

// LocaleVars are consulted in this order; the first non-empty one decides.
var LocaleVars = []string{"LC_ALL", "LC_MESSAGES", "LANG"}

// FromLocale picks the language from the POSIX locale variables. It returns
// the language and a description of the deciding variable ("LANG=ja_JP.UTF-8",
// or "" when none was set).
func FromLocale(getenv func(string) string) (Lang, string) {
	for _, v := range LocaleVars {
		if val := getenv(v); val != "" {
			if IsJapaneseLocale(val) {
				return JA, v + "=" + val
			}
			return EN, v + "=" + val
		}
	}
	return Default, ""
}

// IsJapaneseLocale reports whether a locale name such as "ja_JP.UTF-8",
// "ja_JP", "ja", "ja_JP.eucJP" or "ja_JP@x" denotes Japanese. "C", "POSIX"
// and every other value are not Japanese.
func IsJapaneseLocale(v string) bool {
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexAny(v, "_-"); i >= 0 {
		v = v[:i]
	}
	return strings.EqualFold(v, "ja")
}

// T returns the message for key in language l, formatted with args (fmt
// verbs). Error arguments are translated too. Unknown keys fall back to
// English, then to the key itself.
func (l Lang) T(key string, args ...any) string {
	msg, ok := catalogs[l][key]
	if !ok {
		msg, ok = catalogs[EN][key]
		if !ok {
			return key
		}
	}
	if len(args) == 0 {
		return msg
	}
	conv := make([]any, len(args))
	for i, a := range args {
		if e, ok := a.(error); ok {
			conv[i] = l.Err(e)
		} else {
			conv[i] = a
		}
	}
	return fmt.Sprintf(msg, conv...)
}

// Err renders err in language l. Errors built with E/Wrap are translated;
// any other error (e.g. raw task output) is shown unchanged.
func (l Lang) Err(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return l.T(e.Key, e.Args...)
	}
	return err.Error()
}

// Error is a translatable error. Error() renders it in English; use
// Lang.Err to show it in the selected language.
type Error struct {
	Key   string
	Args  []any
	Cause error
}

func (e *Error) Error() string { return EN.T(e.Key, e.Args...) }

// Unwrap returns the cause (so errors.Is works with sentinel errors).
func (e *Error) Unwrap() error { return e.Cause }

// E builds a translatable error.
func E(key string, args ...any) *Error { return &Error{Key: key, Args: args} }

// Wrap builds a translatable error that wraps cause.
func Wrap(cause error, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, Cause: cause}
}

// Keys returns the sorted message keys of language l.
func Keys(l Lang) []string {
	out := make([]string, 0, len(catalogs[l]))
	for k := range catalogs[l] {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Has reports whether key exists in language l.
func Has(l Lang, key string) bool {
	_, ok := catalogs[l][key]
	return ok
}

// Raw returns the unformatted catalog entry (for tests).
func Raw(l Lang, key string) string { return catalogs[l][key] }

var catalogs = map[Lang]map[string]string{EN: en, JA: ja}
