// Package tw talks to Taskwarrior exclusively through the `task` command
// (export / add / modify / done / delete). It never touches Taskwarrior's
// database files directly.
package tw

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// UDA names used by Jot. They are passed to every `task` invocation as
// rc.uda.<name>.type overrides, so the user's taskrc does not need to change.
const (
	UDAToday = "jottoday" // string: local date (YYYY-MM-DD) the task was put on Today
	UDAOrder = "jotorder" // numeric: manual order inside Today (ascending)
	UDANote  = "jotnote"  // string: multi-line memo
)

// Annotation is a Taskwarrior annotation.
type Annotation struct {
	Entry       string `json:"entry"`
	Description string `json:"description"`
}

// Task is the subset of a Taskwarrior task Jot uses. Raw keeps every field
// as exported so nothing is lost.
type Task struct {
	ID          int          `json:"id"`
	UUID        string       `json:"uuid"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	Project     string       `json:"project"`
	Tags        []string     `json:"tags"`
	Entry       string       `json:"entry"`
	End         string       `json:"end"`
	Due         string       `json:"due"`
	Scheduled   string       `json:"scheduled"`
	Wait        string       `json:"wait"`
	Annotations []Annotation `json:"annotations"`
	Urgency     float64      `json:"urgency"`

	Today    string  `json:"-"`
	Order    float64 `json:"-"`
	HasOrder bool    `json:"-"`
	Note     string  `json:"-"`
}

// UnmarshalJSON decodes standard fields plus Jot's UDAs (which may arrive as
// strings or numbers depending on whether the UDA is defined).
func (t *Task) UnmarshalJSON(b []byte) error {
	type plain Task
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*t = Task(p)
	t.Today = rawString(raw[UDAToday])
	t.Note = rawString(raw[UDANote])
	if o := rawString(raw[UDAOrder]); o != "" {
		if f, err := strconv.ParseFloat(o, 64); err == nil {
			t.Order, t.HasOrder = f, true
		}
	}
	return nil
}

func rawString(m json.RawMessage) string {
	if len(m) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(m, &s); err == nil {
		return s
	}
	var f float64
	if err := json.Unmarshal(m, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strings.Trim(string(m), `"`)
}

// ParseTime parses Taskwarrior's basic ISO format (20261124T081013Z).
func ParseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// IsPending reports whether the task is open (pending or waiting).
func (t Task) IsPending() bool { return t.Status == "pending" || t.Status == "waiting" }

// IsCompleted reports whether the task is completed.
func (t Task) IsCompleted() bool { return t.Status == "completed" }

// HasTag reports whether tag is set.
func (t Task) HasTag(tag string) bool {
	for _, x := range t.Tags {
		if x == tag {
			return true
		}
	}
	return false
}
