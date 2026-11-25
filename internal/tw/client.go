package tw

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/takeshiD/jot/internal/i18n"
)

// Client runs the Taskwarrior CLI. All reads use `task export`; all writes use
// `task <uuid> modify|done|delete`, `task add` and (only as a fallback for
// free text that the CLI parser cannot round-trip) `task import`.
type Client struct {
	Bin string // path or name of the task binary (default "task")
}

// ErrNotInstalled is returned when the task binary cannot be executed.
var ErrNotInstalled = i18n.E("tw.not_installed")

// Error is a failed task invocation.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := firstLine(e.Stderr)
	if msg == "" {
		msg = e.Err.Error()
	}
	// Cause first so it stays visible on narrow status lines; UUIDs shortened.
	var short []string
	for i, a := range e.Args {
		if i >= 3 {
			short = append(short, "...")
			break
		}
		if uuidRe.MatchString(a) {
			a = a[:8]
		}
		if len(a) > 24 {
			a = a[:24] + "~"
		}
		short = append(short, a)
	}
	return "task: " + msg + " (task " + strings.Join(short, " ") + ")"
}

func (e *Error) Unwrap() error { return e.Err }

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "TASKRC override") || strings.HasPrefix(l, "TASKDATA override") || strings.HasPrefix(l, "Configuration override") {
			continue
		}
		return l
	}
	return ""
}

// New returns a client for the given binary ("" means "task").
func New(bin string) *Client {
	if bin == "" {
		bin = "task"
	}
	return &Client{Bin: bin}
}

// RCOverrides are passed to every task invocation. UDAs are defined here so
// the user's taskrc is never modified.
func RCOverrides() []string {
	return []string{
		"rc.confirmation=off",
		"rc.recurrence.confirmation=no",
		"rc.color=off",
		"rc.json.array=on",
		"rc.uda." + UDAToday + ".type=string",
		"rc.uda." + UDAToday + ".label=Today",
		"rc.uda." + UDAOrder + ".type=numeric",
		"rc.uda." + UDAOrder + ".label=Order",
		"rc.uda." + UDANote + ".type=string",
		"rc.uda." + UDANote + ".label=Note",
	}
}

// TaskrcSnippet is what users may add to ~/.taskrc so plain `task` commands
// know Jot's UDAs (optional; export shows them regardless).
func TaskrcSnippet() string {
	return "# Jot UDAs (optional: lets `task` reports/filters know these fields)\n" +
		"uda." + UDAToday + ".type=string\nuda." + UDAToday + ".label=Today\n" +
		"uda." + UDAOrder + ".type=numeric\nuda." + UDAOrder + ".label=Order\n" +
		"uda." + UDANote + ".type=string\nuda." + UDANote + ".label=Note\n"
}

func (c *Client) run(stdin []byte, verbose string, args ...string) (string, error) {
	full := append(RCOverrides(), "rc.verbose="+verbose)
	full = append(full, args...)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		cmd := exec.Command(c.Bin, full...)
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if stdin != nil {
			cmd.Stdin = bytes.NewReader(stdin)
		}
		err := cmd.Run()
		if err == nil {
			return out.String(), nil
		}
		if notFound(err) {
			return "", i18n.Wrap(ErrNotInstalled, "tw.not_installed_hint", c.Bin)
		}
		lastErr = &Error{Args: args, Stderr: errb.String() + "\n" + out.String(), Err: err}
		if !strings.Contains(strings.ToLower(errb.String()), "locked") {
			break
		}
		time.Sleep(time.Duration(150*(attempt+1)) * time.Millisecond)
	}
	return "", lastErr
}

// Version returns `task --version`.
func (c *Client) Version() (string, error) {
	out, err := exec.Command(c.Bin, "--version").Output()
	if err != nil {
		var ee *exec.Error
		if errors.As(err, &ee) {
			return "", i18n.Wrap(ErrNotInstalled, "tw.not_installed_bin", c.Bin)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseExport(out string) ([]Task, error) {
	i := strings.Index(out, "[")
	if i < 0 {
		return nil, i18n.E("tw.export_unparsable", firstLine(out))
	}
	var ts []Task
	if err := json.Unmarshal([]byte(out[i:]), &ts); err != nil {
		return nil, i18n.Wrap(err, "tw.export_json", err)
	}
	return ts, nil
}

// Export returns all non-deleted tasks.
func (c *Client) Export() ([]Task, error) {
	out, err := c.run(nil, "nothing", "status.not:deleted", "export")
	if err != nil {
		return nil, err
	}
	return parseExport(out)
}

// Get returns one task by UUID (any status).
func (c *Client) Get(uuid string) (Task, error) {
	if err := checkUUID(uuid); err != nil {
		return Task{}, err
	}
	out, err := c.run(nil, "nothing", "uuid:"+uuid, "export")
	if err != nil {
		return Task{}, err
	}
	ts, err := parseExport(out)
	if err != nil {
		return Task{}, err
	}
	if len(ts) == 0 {
		return Task{}, i18n.E("tw.task_not_found", short(uuid))
	}
	return ts[0], nil
}

func short(uuid string) string {
	if len(uuid) > 8 {
		return uuid[:8]
	}
	return uuid
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var createdRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func checkUUID(u string) error {
	if !uuidRe.MatchString(u) {
		return i18n.E("tw.bad_uuid", u)
	}
	return nil
}

// Add creates a pending task and returns its UUID. Everything after "--" is
// taken literally as the description by Taskwarrior.
func (c *Client) Add(title string, mods ...string) (string, error) {
	title = CleanTitle(title)
	if title == "" {
		return "", i18n.E("tw.empty_title")
	}
	args := append([]string{"add"}, mods...)
	args = append(args, "--", title)
	out, err := c.run(nil, "new-uuid", args...)
	if err != nil {
		return "", err
	}
	u := createdRe.FindString(out)
	if u == "" {
		return "", i18n.E("tw.no_uuid", firstLine(out))
	}
	// Taskwarrior may still have reinterpreted parts; verify and repair.
	if err := c.SetText(u, "description", title); err != nil {
		return u, err
	}
	return u, nil
}

// Modify runs `task <uuid> modify args...`.
func (c *Client) Modify(uuid string, args ...string) error {
	if err := checkUUID(uuid); err != nil {
		return err
	}
	_, err := c.run(nil, "nothing", append([]string{uuid, "modify"}, args...)...)
	return err
}

// Done marks the task completed.
func (c *Client) Done(uuid string) error {
	if err := checkUUID(uuid); err != nil {
		return err
	}
	_, err := c.run(nil, "nothing", uuid, "done")
	return err
}

// Undone moves a completed task back to pending.
func (c *Client) Undone(uuid string) error { return c.Modify(uuid, "status:pending") }

// Delete marks the task deleted.
func (c *Client) Delete(uuid string) error {
	if err := checkUUID(uuid); err != nil {
		return err
	}
	_, err := c.run(nil, "nothing", uuid, "delete")
	return err
}

// CleanTitle turns a title into a single trimmed line.
func CleanTitle(s string) string {
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(s)
	return strings.TrimSpace(s)
}

// SetText sets a free-text attribute (description or jotnote) exactly.
// It first uses `task <uuid> modify field:value` (one argv element, no
// shell). If the stored value differs (the CLI parser rewrote it), it falls
// back to read-modify-write via `task export` + `task import` of the full
// task JSON so that no other field is lost.
func (c *Client) SetText(uuid, field, value string) error {
	if field == "description" {
		value = CleanTitle(value)
		if value == "" {
			return i18n.E("tw.empty_title")
		}
	}
	before, err := c.getRaw(uuid)
	if err != nil {
		return err
	}
	want := map[string]any{}
	for k, v := range before {
		want[k] = v
	}
	if value == "" {
		delete(want, field)
	} else {
		want[field] = value
	}
	if err := c.Modify(uuid, field+":"+value); err != nil {
		return err
	}
	got, err := c.getRaw(uuid)
	if err != nil {
		return err
	}
	if sameTask(got, want) {
		return nil
	}
	// The CLI parser changed something else (or not exactly the value):
	// restore the snapshot with the new value via import.
	delete(want, "id")
	delete(want, "urgency")
	delete(want, "modified")
	b, err := json.Marshal(want)
	if err != nil {
		return err
	}
	_, err = c.run(b, "nothing", "import", "-")
	return err
}

func (c *Client) getRaw(uuid string) (map[string]any, error) {
	out, err := c.run(nil, "nothing", "uuid:"+uuid, "export")
	if err != nil {
		return nil, err
	}
	i := strings.Index(out, "[")
	if i < 0 {
		return nil, i18n.E("tw.export_unparsable", firstLine(out))
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out[i:]), &arr); err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, i18n.E("tw.task_not_found", short(uuid))
	}
	return arr[0], nil
}

var tagRe = regexp.MustCompile(`^[^\s+\-:()"'\\][^\s:()"'\\]*$`)

// ValidTag reports whether a tag name is safe to pass as +tag/-tag.
func ValidTag(t string) bool { return tagRe.MatchString(t) && !strings.HasPrefix(t, "-") }

// ValidProject reports whether a project name is safe for project:NAME.
func ValidProject(p string) bool {
	return p == "" || (!strings.ContainsAny(p, " \t\n\r\"'\\()") && !strings.HasPrefix(p, "-"))
}

// sameTask compares two exported tasks ignoring volatile fields.
func sameTask(a, b map[string]any) bool {
	skip := map[string]bool{"id": true, "urgency": true, "modified": true}
	for k, v := range a {
		if skip[k] {
			continue
		}
		bv, ok := b[k]
		if !ok {
			return false
		}
		x, _ := json.Marshal(v)
		y, _ := json.Marshal(bv)
		if string(x) != string(y) {
			return false
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok && !skip[k] {
			return false
		}
	}
	return true
}

func notFound(err error) bool {
	var ee *exec.Error
	return errors.As(err, &ee) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, exec.ErrNotFound)
}
