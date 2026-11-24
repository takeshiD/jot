package ui

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Immediate wraps the model with a synchronous renderer: every Update is
// drawn right away (only changed lines are rewritten). Bubble Tea's own
// renderer is frame-ticker based (≤120 fps), which leaves the screen stale
// for several milliseconds after each key press; Jot instead paints as soon
// as the input has been handled. Run the program with tea.WithoutRenderer().
type Immediate struct {
	m    *Model
	out  io.Writer
	last []string
	w, h int
}

// NewImmediate returns the wrapper writing to out (normally os.Stdout).
func NewImmediate(m *Model, out io.Writer) *Immediate { return &Immediate{m: m, out: out} }

// Enter switches to the alternate screen, hides the cursor and enables
// bracketed paste. Leave undoes it.
func (r *Immediate) Enter() { io.WriteString(r.out, "\x1b[?1049h\x1b[?25l\x1b[?2004h\x1b[2J") }

// Leave restores the normal screen.
func (r *Immediate) Leave() { io.WriteString(r.out, "\x1b[0m\x1b[?2004l\x1b[?25h\x1b[?1049l") }

// Init implements tea.Model.
func (r *Immediate) Init() tea.Cmd {
	cmd := r.m.Init()
	r.paint()
	return cmd
}

// Update implements tea.Model.
func (r *Immediate) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := r.m.Update(msg)
	if r.m.quitting {
		return r, cmd
	}
	r.paint()
	return r, cmd
}

// View implements tea.Model (unused: rendering happens in paint).
func (r *Immediate) View() string { return "" }

func (r *Immediate) paint() {
	lines := strings.Split(r.m.View(), "\n")
	var b bytes.Buffer
	if r.w != r.m.width || r.h != r.m.height {
		r.w, r.h, r.last = r.m.width, r.m.height, nil
		b.WriteString("\x1b[0m\x1b[2J")
	}
	for i := 0; i < r.h; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		if r.last != nil && i < len(r.last) && r.last[i] == line {
			continue
		}
		fmt.Fprintf(&b, "\x1b[%d;1H\x1b[0m%s\x1b[0m\x1b[K", i+1, line)
	}
	r.last = lines
	if b.Len() > 0 {
		r.out.Write(b.Bytes())
	}
}
