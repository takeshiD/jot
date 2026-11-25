// Package urls extracts links from memo text and opens them safely.
package urls

import (
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"github.com/takeshiD/jot/internal/i18n"
)

// Link is a URL-like token found in text.
type Link struct {
	Raw    string
	Safe   bool  // only safe links can be opened
	Reason error // why it is blocked (translatable; see i18n.Lang.Err)
}

// Anything that looks like scheme:... (scheme://, or the dangerous
// scheme-only forms javascript:, data:, vbscript:, file:).
var candRe = regexp.MustCompile(`(?i)\b(?:[a-z][a-z0-9+.\-]*://[^\s<>"'` + "`" + `]+|(?:javascript|data|vbscript|file):[^\s<>"'` + "`" + `]+)`)

// Extract returns links in order of appearance, without duplicates.
func Extract(text string) []Link {
	var out []Link
	seen := map[string]bool{}
	for _, m := range candRe.FindAllString(text, -1) {
		m = trimTrailing(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		err := Check(m)
		l := Link{Raw: m, Safe: err == nil}
		if err != nil {
			l.Reason = err
		}
		out = append(out, l)
	}
	return out
}

// Check validates a URL for opening: http/https only, with a host, no
// whitespace/control characters, and no leading '-'.
func Check(raw string) error {
	if raw == "" {
		return i18n.E("url.empty")
	}
	for _, r := range raw {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return i18n.E("url.control")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return i18n.E("url.unparsable")
	}
	s := strings.ToLower(u.Scheme)
	if s != "http" && s != "https" {
		return i18n.E("url.scheme", u.Scheme)
	}
	if u.Host == "" || u.Hostname() == "" {
		return i18n.E("url.no_host")
	}
	return nil
}

// Command builds the argv that opens raw. cmdline is the configured command
// (e.g. ["firefox", "--new-tab"] or ["chromium", "{url}"]); empty means the
// OS default opener. The URL is always a single argv element; no shell is
// involved.
func Command(cmdline []string, raw string) ([]string, error) {
	if err := Check(raw); err != nil {
		return nil, err
	}
	if len(cmdline) == 0 {
		switch runtime.GOOS {
		case "darwin":
			cmdline = []string{"open"}
		case "windows":
			cmdline = []string{"rundll32", "url.dll,FileProtocolHandler"}
		default:
			cmdline = []string{"xdg-open"}
		}
	}
	argv := make([]string, 0, len(cmdline)+1)
	replaced := false
	for _, a := range cmdline {
		if a == "{url}" {
			argv = append(argv, raw)
			replaced = true
		} else {
			argv = append(argv, a)
		}
	}
	if !replaced {
		argv = append(argv, raw)
	}
	return argv, nil
}

// Open starts the opener for raw in the background (stdin/stdout/stderr are
// not connected to the terminal) and returns the argv used.
func Open(cmdline []string, raw string) ([]string, error) {
	argv, err := Command(cmdline, raw)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return argv, i18n.Wrap(err, "url.launch_failed", argv[0], err)
	}
	go func() { _ = cmd.Wait() }()
	return argv, nil
}

// trimTrailing removes sentence punctuation after a URL, and closing
// brackets that have no matching opening bracket inside the URL.
func trimTrailing(m string) string {
	pairs := map[rune]rune{')': '(', ']': '[', '}': '{', '）': '（', '」': '「', '>': '<'} // i18n:ignore (punctuation, not UI text)
	for m != "" {
		r := []rune(m)
		last := r[len(r)-1]
		if strings.ContainsRune(".,;:!?、。", last) { // i18n:ignore (punctuation, not UI text)
			m = string(r[:len(r)-1])
			continue
		}
		if open, ok := pairs[last]; ok && strings.Count(m, string(open)) < strings.Count(m, string(last)) {
			m = string(r[:len(r)-1])
			continue
		}
		break
	}
	return m
}
