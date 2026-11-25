package tw_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tkcd/jot/internal/i18n"
	"github.com/tkcd/jot/internal/testenv"
	"github.com/tkcd/jot/internal/tw"
)

// Jot writes -> plain `task export` sees the same content.
func TestRoundTripJotToCLI(t *testing.T) {
	testenv.Setup(t)
	c := testenv.Client()
	title := `+x project:y due:tomorrow -- "引用" (括弧) 日本語のタイトル`
	u, err := c.Add(title)
	if err != nil {
		t.Fatal(err)
	}
	got := testenv.ByUUID(t, u)
	if got["description"] != title {
		t.Fatalf("description = %q", got["description"])
	}
	if _, ok := got["project"]; ok {
		t.Fatalf("title text must not be parsed as project: %v", got)
	}
	note := "1行目 https://example.com/a?b=c&d=e\n\n3行目 description:zzz +tag project:p -- end"
	if err := c.SetText(u, tw.UDANote, note); err != nil {
		t.Fatal(err)
	}
	if err := c.Modify(u, "+work", "+日本語タグ", "project:home"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetText(u, "description", "新しいタイトル"); err != nil {
		t.Fatal(err)
	}
	got = testenv.ByUUID(t, u)
	if got[tw.UDANote] != note {
		t.Fatalf("note = %q", got[tw.UDANote])
	}
	if got["description"] != "新しいタイトル" || got["project"] != "home" {
		t.Fatalf("got %v", got)
	}
	tags := got["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("tags %v", tags)
	}
	// plain `task list` shows the task with its tag filter
	if out := testenv.Task(t, "+work", "list"); !strings.Contains(out, "新しいタイトル") {
		t.Fatalf("task +work list: %q", out)
	}
	if err := c.Modify(u, "-work"); err != nil {
		t.Fatal(err)
	}
	if testenv.ByUUID(t, u)["tags"].([]any)[0] != "日本語タグ" {
		t.Fatal("tag not removed")
	}
	// done -> completed; undone -> pending; delete -> deleted
	if err := c.Done(u); err != nil {
		t.Fatal(err)
	}
	if s := testenv.ByUUID(t, u)["status"]; s != "completed" {
		t.Fatalf("status %v", s)
	}
	if err := c.Undone(u); err != nil {
		t.Fatal(err)
	}
	got = testenv.ByUUID(t, u)
	if got["status"] != "pending" || got["end"] != nil {
		t.Fatalf("after undone: %v", got)
	}
	if got[tw.UDANote] != note || got["project"] != "home" {
		t.Fatal("other fields lost")
	}
	if err := c.Delete(u); err != nil {
		t.Fatal(err)
	}
	if s := testenv.ByUUID(t, u)["status"]; s != "deleted" {
		t.Fatalf("status %v", s)
	}
}

// CLI writes -> Jot's Export sees them.
func TestRoundTripCLIToJot(t *testing.T) {
	testenv.Setup(t)
	u := testenv.AddCLI(t, "project:cli", "+fromcli", "CLIで作ったタスク")
	testenv.Task(t, u, "annotate", "see https://example.org/x")
	testenv.Task(t, "rc.uda.jottoday.type=string", u, "modify", "jottoday:2026-11-24")
	ts, err := testenv.Client().Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 {
		t.Fatalf("%d tasks", len(ts))
	}
	x := ts[0]
	if x.UUID != u || x.Description != "CLIで作ったタスク" || x.Project != "cli" || !x.HasTag("fromcli") || x.Today != "2026-11-24" {
		t.Fatalf("%+v", x)
	}
	if len(x.Annotations) != 1 || !strings.Contains(x.Annotations[0].Description, "https://example.org/x") {
		t.Fatalf("annotations %+v", x.Annotations)
	}
	testenv.Task(t, u, "modify", "Renamed")
	testenv.Task(t, u, "done")
	ts, _ = testenv.Client().Export()
	if ts[0].Description != "Renamed" || ts[0].Status != "completed" {
		t.Fatalf("%+v", ts[0])
	}
}

// Free text that the CLI parser would rewrite still round-trips exactly
// (SetText falls back to export+import, preserving other fields).
func TestSetTextExactWithFallback(t *testing.T) {
	testenv.Setup(t)
	c := testenv.Client()
	u, err := c.Add("base", "project:keep", "+keep", "due:2030-01-01")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"description:line1\nline2", "  leading and trailing  ", "a\tb\\c 'q' \"dq\"", "rc.color=on", ""} {
		if err := c.SetText(u, tw.UDANote, v); err != nil {
			t.Fatal(err)
		}
		got := testenv.ByUUID(t, u)
		nv, _ := got[tw.UDANote].(string)
		if nv != v {
			t.Fatalf("note %q -> %q", v, nv)
		}
		if got["project"] != "keep" || got["due"] == nil || got["description"] != "base" {
			t.Fatalf("lost fields: %v", got)
		}
	}
}

// A missing task binary yields a friendly error, not a panic.
func TestMissingBinary(t *testing.T) {
	c := tw.New(filepath.Join(t.TempDir(), "no-such-task"))
	_, err := c.Export()
	if err == nil || !strings.Contains(i18n.JA.Err(err), "見つかりません") {
		t.Fatalf("err = %v", err)
	}
}

// task failing (no rc file) is reported with its message.
func TestTaskErrorMessage(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip()
	}
	dir := testenv.Setup(t)
	t.Setenv("TASKRC", filepath.Join(dir, "missing-rc"))
	_, err := testenv.Client().Export()
	if err == nil {
		t.Fatal("expected error")
	}
	t.Log(err)
}

func TestValidators(t *testing.T) {
	for _, ok := range []string{"work", "日本語", "a.b", "x_y"} {
		if !tw.ValidTag(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "a b", "-x", "+x", "a:b", "(x)"} {
		if tw.ValidTag(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
	if tw.CleanTitle(" a\nb\tc ") != "a b c" {
		t.Error("CleanTitle")
	}
}

// Errors show task's own message first and a shortened command, so the
// cause stays visible on an 80-column status line.
func TestErrorMessageCauseFirst(t *testing.T) {
	testenv.Setup(t)
	u := "01234567-89ab-cdef-0123-456789abcdef"
	err := testenv.Client().Modify(u, "jottoday:2026-11-24", "jotorder:3")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "task: ") || strings.Contains(msg, u) || !strings.Contains(msg, "01234567") {
		t.Fatalf("msg = %q", msg)
	}
	if len([]rune(msg)) > 120 {
		t.Fatalf("too long: %q", msg)
	}
	t.Log(msg)
}
