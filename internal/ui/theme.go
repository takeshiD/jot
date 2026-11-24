package ui

import "github.com/charmbracelet/lipgloss"

// Theme is a palette. Colors are ANSI-256 indexes so they render the same
// on 256-color and truecolor terminals.
type Theme struct {
	Name   string
	FG     string // normal text
	Muted  string // secondary text, hints
	Accent string // active tab, key names
	BarBG  string // header/footer bar background
	BarFG  string
	SelBG  string // selected row background
	SelFG  string
	Today  string // Today marker / carried-over
	Tag    string
	Group  string // group (project) headers
	Done   string // completed tasks
	Error  string
	OK     string
}

// Themes bundled with Jot.
var Themes = map[string]Theme{
	"dark": {Name: "dark", FG: "252", Muted: "244", Accent: "75", BarBG: "236", BarFG: "252",
		SelBG: "24", SelFG: "231", Today: "214", Tag: "114", Group: "176", Done: "242", Error: "203", OK: "78"},
	"light": {Name: "light", FG: "235", Muted: "243", Accent: "26", BarBG: "253", BarFG: "235",
		SelBG: "153", SelFG: "16", Today: "166", Tag: "28", Group: "91", Done: "248", Error: "160", OK: "28"},
	"dusk": {Name: "dusk", FG: "187", Muted: "101", Accent: "38", BarBG: "23", BarFG: "187",
		SelBG: "60", SelFG: "230", Today: "178", Tag: "107", Group: "168", Done: "66", Error: "167", OK: "107"},
}

// ThemeNames lists bundled theme names in display order.
var ThemeNames = []string{"dark", "light", "dusk"}

func (t Theme) c(s string) lipgloss.Color { return lipgloss.Color(s) }

// Style returns a foreground style, optionally on the selection background.
func (t Theme) fg(color string, sel bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(t.c(color))
	if sel {
		s = s.Background(t.c(t.SelBG))
	}
	return s
}
