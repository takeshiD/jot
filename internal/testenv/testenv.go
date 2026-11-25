// Package testenv creates isolated Taskwarrior datasets for tests. It points
// TASKRC/TASKDATA (and HOME) at a temp dir so the user's ~/.taskrc and
// ~/.task are never read or modified.
package testenv

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takeshiD/jot/internal/tw"
)

// Setup isolates Taskwarrior for the test and returns the temp dir. Tests
// are skipped when the task binary is unavailable.
func Setup(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task (Taskwarrior) not installed")
	}
	dir := t.TempDir()
	rc := filepath.Join(dir, "taskrc")
	if err := os.WriteFile(rc, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "home", ".config"))
	t.Setenv("TASKRC", rc)
	t.Setenv("TASKDATA", filepath.Join(dir, "data"))
	return dir
}

// Task runs the plain task CLI (as a user would, WITHOUT Jot's rc overrides)
// and returns stdout.
func Task(t *testing.T, args ...string) string {
	t.Helper()
	full := append([]string{"rc.confirmation=off", "rc.verbose=nothing", "rc.color=off"}, args...)
	out, err := exec.Command("task", full...).Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("task %s: %v %s", strings.Join(args, " "), err, stderr)
	}
	return string(out)
}

// Export runs plain `task <filter> export` and decodes raw JSON objects.
func Export(t *testing.T, filter ...string) []map[string]any {
	t.Helper()
	out := Task(t, append(filter, "export")...)
	i := strings.Index(out, "[")
	if i < 0 {
		t.Fatalf("no JSON in export: %q", out)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out[i:]), &arr); err != nil {
		t.Fatal(err)
	}
	return arr
}

// ByUUID returns the exported object for uuid.
func ByUUID(t *testing.T, uuid string) map[string]any {
	t.Helper()
	arr := Export(t, "uuid:"+uuid)
	if len(arr) != 1 {
		t.Fatalf("uuid %s: %d results", uuid, len(arr))
	}
	return arr[0]
}

// AddCLI creates a task with the plain CLI and returns its UUID.
func AddCLI(t *testing.T, args ...string) string {
	t.Helper()
	full := append([]string{"rc.confirmation=off", "rc.verbose=new-uuid", "add"}, args...)
	out, err := exec.Command("task", full...).Output()
	if err != nil {
		t.Fatalf("task add: %v", err)
	}
	f := strings.Fields(string(out))
	for _, w := range f {
		w = strings.TrimSuffix(w, ".")
		if len(w) == 36 && strings.Count(w, "-") == 4 {
			return w
		}
	}
	t.Fatalf("no uuid in %q", out)
	return ""
}

// Client returns a Jot Taskwarrior client.
func Client() *tw.Client { return tw.New("task") }
