package urls

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestExtract(t *testing.T) {
	text := "資料 https://example.com/a?x=1&y=2 と http://foo.test/パス。\n" +
		"危険: javascript:alert(1) file:///etc/passwd ftp://x.y/z data:text/html,hi\n" +
		"重複 https://example.com/a?x=1&y=2, (https://b.example/c) https://w.example/A_(B)."
	got := Extract(text)
	var safe, blocked []string
	for _, l := range got {
		if l.Safe {
			safe = append(safe, l.Raw)
		} else {
			blocked = append(blocked, l.Raw)
		}
	}
	wantSafe := []string{"https://example.com/a?x=1&y=2", "http://foo.test/パス", "https://b.example/c", "https://w.example/A_(B)"}
	if !reflect.DeepEqual(safe, wantSafe) {
		t.Fatalf("safe = %q", safe)
	}
	if len(blocked) != 4 {
		t.Fatalf("blocked = %q", blocked)
	}
}

func TestCheck(t *testing.T) {
	for _, ok := range []string{"https://a.b", "HTTP://A.B/x", "https://例え.jp/パス"} {
		if err := Check(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "javascript:alert(1)", "file:///etc/passwd", "https://", "http:///x", "https://a.b/\nx", "https://a b", "-https://a.b", "ftp://a.b"} {
		if Check(bad) == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestCommandNoShell(t *testing.T) {
	u := "https://example.com/?q=$(id)&a=`x`;b"
	argv, err := Command([]string{"firefox", "--new-tab"}, u)
	if err != nil || !reflect.DeepEqual(argv, []string{"firefox", "--new-tab", u}) {
		t.Fatal(argv, err)
	}
	argv, _ = Command([]string{"chromium", "{url}", "--incognito"}, u)
	if !reflect.DeepEqual(argv, []string{"chromium", u, "--incognito"}) {
		t.Fatal(argv)
	}
	argv, _ = Command(nil, u)
	if len(argv) != 2 || argv[1] != u {
		t.Fatal(argv)
	}
	if _, err := Command([]string{"x"}, "javascript:alert(1)"); err == nil {
		t.Fatal("dangerous URL accepted")
	}
}

// Open really executes the configured command with the URL as one argument.
func TestOpenRecordsArgs(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	t.Setenv("JOT_OPEN_LOG", log)
	u := "https://example.com/a b?c=$(id)"
	u = strings.ReplaceAll(u, " ", "%20")
	if _, err := Open([]string{"sh", "../../scripts/record-open.sh"}, u); err != nil {
		t.Fatal(err)
	}
	var b []byte
	for i := 0; i < 50; i++ {
		b, _ = os.ReadFile(log)
		if strings.Contains(string(b), "END") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if string(b) != "ARG:"+u+"\nEND\n" {
		t.Fatalf("log = %q", b)
	}
}
