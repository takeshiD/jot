package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/tkcd/jot/internal/config"
	"github.com/tkcd/jot/internal/today"
	"github.com/tkcd/jot/internal/tw"
)

// Glyphs avoid East-Asian "ambiguous width" characters so that layouts stay
// aligned in CJK locales.
const (
	glyphCursor  = "›"
	glyphToday   = "☀"
	glyphOpen    = "☐"
	glyphDone    = "✓"
	glyphCarried = "↻"
	tail         = "~"
)

// width returns the display width of s (CJK = 2).
func width(s string) int { return ansi.StringWidth(s) }

// trunc cuts plain text s to at most w cells.
func trunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, tail)
}

// pad right-pads plain text to exactly w cells (truncating if needed).
func pad(s string, w int) string {
	s = trunc(s, w)
	if d := w - width(s); d > 0 {
		s += strings.Repeat(" ", d)
	}
	return s
}

func oneLine(s string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(s)
}

// View implements tea.Model.
func (m *Model) View() string {
	w, h := m.width, m.height
	if w < 10 || h < 4 {
		return trunc("Jot: 端末が小さすぎます", w)
	}
	var lines []string
	lines = append(lines, m.headerLine(w))
	bodyH := h - 3
	var body []string
	switch m.mode {
	case modeHelp:
		body = m.helpLines(w, bodyH)
	case modeNote:
		body = m.noteLines(w, bodyH)
	case modeURL:
		body = m.urlLines(w, bodyH)
	case modeDetail:
		if t := m.findTask(m.detailUUID); t != nil {
			body = m.detailLines(*t, w, bodyH)
		} else {
			m.mode = modeList
			body = m.listLines(w, bodyH)
		}
	default:
		body = m.listLines(w, bodyH)
	}
	for len(body) < bodyH {
		body = append(body, "")
	}
	lines = append(lines, body[:bodyH]...)
	lines = append(lines, m.statusLine(w), m.hintLine(w))
	return strings.Join(lines, "\n")
}

func (m *Model) bar(w int) lipgloss.Style {
	return lipgloss.NewStyle().Background(m.theme.c(m.theme.BarBG)).Foreground(m.theme.c(m.theme.BarFG))
}

func (m *Model) headerLine(w int) string {
	th := m.theme
	bar := m.bar(w)
	tab := func(label string, active bool) string {
		if active {
			return bar.Bold(true).Foreground(th.c(th.Accent)).Underline(true).Render(label)
		}
		return bar.Foreground(th.c(th.Muted)).Render(label)
	}
	nToday := len(m.todayList())
	left := []struct {
		s      string
		styled string
	}{
		{" Jot ", bar.Bold(true).Render(" Jot ")},
		{" ", bar.Render(" ")},
	}
	todayLabel := fmt.Sprintf("Today %d", nToday)
	allLabel := fmt.Sprintf("全タスク %d", m.countAll())
	left = append(left,
		struct{ s, styled string }{todayLabel, tab(todayLabel, m.view == viewToday)},
		struct{ s, styled string }{"  ", bar.Render("  ")},
		struct{ s, styled string }{allLabel, tab(allLabel, m.view == viewAll)},
	)
	now := m.now()
	date := m.day + " " + weekday(now) + " "
	if w < 50 {
		date = m.day[5:] + " "
	}
	var b strings.Builder
	used := 0
	for _, p := range left {
		pw := width(p.s)
		if used+pw > w {
			b.WriteString(bar.Render(trunc(p.s, w-used)))
			used = w
			break
		}
		b.WriteString(p.styled)
		used += pw
	}
	if used+width(date) <= w {
		b.WriteString(bar.Render(strings.Repeat(" ", w-used-width(date))))
		b.WriteString(bar.Foreground(th.c(th.Muted)).Render(date))
	} else if used < w {
		b.WriteString(bar.Render(strings.Repeat(" ", w-used)))
	}
	return b.String()
}

func weekday(t time.Time) string {
	return "(" + []string{"日", "月", "火", "水", "木", "金", "土"}[t.Weekday()] + ")"
}

// listLines renders the task list with scrolling.
func (m *Model) listLines(w, h int) []string {
	if len(m.rows) == 0 {
		msg := "全タスク: pending のタスクはありません。a で追加できます。"
		if m.view == viewToday {
			msg = "Todayは空です。tab で全タスクへ移り、space でTodayに追加 (a で新規作成)。"
		}
		st := lipgloss.NewStyle().Foreground(m.theme.c(m.theme.Muted))
		return wrapLines(msg, w-2, func(s string) string { return "  " + st.Render(s) })
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
		if m.offset > 0 && m.rows[m.offset-1].task == nil {
			m.offset--
		}
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	if m.offset > len(m.rows)-h {
		m.offset = max(0, len(m.rows)-h)
	}
	var out []string
	for i := m.offset; i < len(m.rows) && len(out) < h; i++ {
		r := m.rows[i]
		if r.task == nil {
			out = append(out, m.groupLine(r, w))
			continue
		}
		pos := 0
		if m.view == viewToday {
			pos = i + 1
		}
		out = append(out, m.taskLine(*r.task, pos, i == m.cursor, w))
	}
	return out
}

func (m *Model) groupLine(r row, w int) string {
	st := lipgloss.NewStyle().Foreground(m.theme.c(m.theme.Group)).Bold(true)
	cnt := fmt.Sprintf(" (%d)", r.count)
	name := trunc("▸ "+r.header, w-width(cnt))
	return st.Render(name) + lipgloss.NewStyle().Foreground(m.theme.c(m.theme.Muted)).Render(cnt)
}

type seg struct {
	text   string
	color  string
	bold   bool
	strike bool
}

// taskLine renders one task in exactly one terminal line of width w.
func (m *Model) taskLine(t tw.Task, pos int, sel bool, w int) string {
	th := m.theme
	st := m.rules.Of(t, m.now())
	cursor := "  "
	if sel {
		cursor = glyphCursor + " "
	}
	var prefix []seg
	prefix = append(prefix, seg{text: cursor, color: th.Accent, bold: true})
	if m.view == viewToday {
		prefix = append(prefix, seg{text: fmt.Sprintf("%2d ", pos), color: th.Muted})
	} else {
		mark := "  "
		if st != today.NotToday {
			mark = glyphToday + " "
		}
		prefix = append(prefix, seg{text: mark, color: th.Today, bold: true})
	}
	check, checkColor := glyphOpen+" ", th.Muted
	titleColor := th.FG
	if t.IsCompleted() {
		check, checkColor, titleColor = glyphDone+" ", th.OK, th.Done
	}
	prefix = append(prefix, seg{text: check, color: checkColor})

	// Right-hand metadata: tags, group (in Today view), carried-over marker.
	var meta []seg
	if st == today.CarriedOver {
		d := t.Today
		if len(d) == 10 {
			d = d[5:7] + "/" + d[8:]
		}
		meta = append(meta, seg{text: glyphCarried + d, color: th.Today})
	}
	if t.Note != "" {
		meta = append(meta, seg{text: "✎", color: th.Muted})
	}
	for _, tg := range t.Tags {
		meta = append(meta, seg{text: "#" + tg, color: th.Tag})
	}
	if m.view == viewToday || !m.grouped {
		if t.Project != "" {
			meta = append(meta, seg{text: "@" + t.Project, color: th.Group})
		}
	}
	if due, ok := tw.ParseTime(t.Due); ok {
		meta = append(meta, seg{text: "期限" + due.Local().Format("01/02"), color: th.Muted})
	}

	pw := 0
	for _, s := range prefix {
		pw += width(s.text)
	}
	avail := w - pw
	title := oneLine(t.Description)
	// Title keeps at least min(titleWidth, 16 cells or half); meta gets the rest.
	minTitle := min(width(title), max(avail/2, min(16, avail)))
	metaW := 0
	var metaKept []seg
	for _, s := range meta {
		sw := width(s.text) + 1
		if avail-(metaW+sw) < minTitle {
			break
		}
		metaKept = append(metaKept, s)
		metaW += sw
	}
	titleW := avail - metaW
	segs := append([]seg{}, prefix...)
	segs = append(segs, seg{text: pad(title, titleW), color: titleColor, bold: st != today.NotToday && m.view == viewAll && t.IsPending(), strike: t.IsCompleted()})
	for _, s := range metaKept {
		s.text = " " + s.text
		segs = append(segs, s)
	}
	var b strings.Builder
	for _, s := range segs {
		style := th.fg(s.color, sel).Bold(s.bold)
		if sel && s.color == th.FG {
			style = style.Foreground(th.c(th.SelFG))
		}
		if s.strike {
			style = style.Strikethrough(true)
		}
		b.WriteString(style.Render(s.text))
	}
	return b.String()
}

func (m *Model) statusLine(w int) string {
	th := m.theme
	if m.mode == modeInput {
		label := inputLabel(m.inKind)
		lab := lipgloss.NewStyle().Foreground(th.c(th.Accent)).Bold(true).Render(trunc(label, w/2))
		return lab + trunc(m.input.View(), w-width(trunc(label, w/2)))
	}
	if m.status == "" {
		return ""
	}
	col := th.OK
	if m.statusErr {
		col = th.Error
	}
	return lipgloss.NewStyle().Foreground(th.c(col)).Render(trunc(oneLine(m.status), w))
}

func (m *Model) keyName(action string) string {
	ks := m.cfg.KeysFor(action)
	if len(ks) == 0 {
		return ""
	}
	return ks[0]
}

func (m *Model) hintLine(w int) string {
	type hint struct{ key, label string }
	var hs []hint
	add := func(action, label string) {
		if k := m.keyName(action); k != "" {
			hs = append(hs, hint{k, label})
		}
	}
	switch m.mode {
	case modeInput:
		hs = []hint{{"enter", "確定"}, {"esc", "取消"}}
	case modeNote:
		hs = []hint{{"ctrl+s", "保存"}, {"esc", "取消"}, {"enter", "改行"}}
	case modeHelp:
		hs = []hint{{"esc", "閉じる"}, {m.keyName("down") + "/" + m.keyName("up"), "スクロール"}}
	case modeURL:
		hs = []hint{{"enter", "開く"}, {"1-9", "番号で開く"}, {"esc", "閉じる"}}
	case modeConfirm:
		hs = []hint{{"y", "削除する"}, {"n/esc", "やめる"}}
	case modeDetail:
		hs = []hint{{"esc", "戻る"}, {m.keyName("down") + "/" + m.keyName("up"), "スクロール"}}
		add("edit_note", "メモ")
		add("open_url", "URL")
		add("edit_title", "タイトル")
		add("edit_tags", "タグ")
		add("toggle_today", "Today")
		add("toggle_done", "完了")
		add("help", "ヘルプ")
	default:
		add("help", "ヘルプ")
		add("quit", "終了")
		add("toggle_today", "Today")
		add("add", "追加")
		add("toggle_done", "完了")
		add("switch_view", "切替")
		if m.view == viewToday {
			add("move_up", "上へ")
			add("move_down", "下へ")
		}
		add("edit_title", "編集")
		add("edit_note", "メモ")
		add("edit_tags", "タグ")
		add("open_url", "URL")
		add("delete", "削除")
	}
	th := m.theme
	bar := m.bar(w)
	keySt := bar.Foreground(th.c(th.Accent)).Bold(true)
	var b strings.Builder
	used := 0
	for i, h := range hs {
		k := h.key
		if k == " " {
			k = "space"
		}
		item := k + " " + h.label
		sep := ""
		if i > 0 {
			sep = "  "
		}
		if used+width(sep+item) > w {
			break
		}
		b.WriteString(bar.Render(sep) + keySt.Render(k) + bar.Render(" "+h.label))
		used += width(sep + item)
	}
	if used < w {
		b.WriteString(bar.Render(strings.Repeat(" ", w-used)))
	}
	return b.String()
}

// wrapLines wraps plain text to width w (CJK aware), applying f to each line.
func wrapLines(text string, w int, f func(string) string) []string {
	var out []string
	if w < 1 {
		w = 1
	}
	for _, para := range strings.Split(text, "\n") {
		para = strings.ReplaceAll(para, "\t", "    ")
		if para == "" {
			out = append(out, f(""))
			continue
		}
		var cur strings.Builder
		cw := 0
		for _, r := range para {
			rw := width(string(r))
			if cw+rw > w && cw > 0 {
				out = append(out, f(cur.String()))
				cur.Reset()
				cw = 0
			}
			cur.WriteRune(r)
			cw += rw
		}
		out = append(out, f(cur.String()))
	}
	return out
}

func (m *Model) detailLines(t tw.Task, w, h int) []string {
	th := m.theme
	label := lipgloss.NewStyle().Foreground(th.c(th.Muted))
	val := lipgloss.NewStyle().Foreground(th.c(th.FG))
	var out []string
	title := lipgloss.NewStyle().Foreground(th.c(th.FG)).Bold(true)
	out = append(out, wrapLines(oneLine(t.Description), w-2, func(s string) string { return " " + title.Render(s) })...)
	field := func(name, v string, color string) {
		if v == "" {
			return
		}
		n := pad(name, 9)
		st := val
		if color != "" {
			st = lipgloss.NewStyle().Foreground(th.c(color))
		}
		out = append(out, " "+label.Render(n)+st.Render(trunc(v, w-10)))
	}
	state := map[today.State]string{today.NotToday: "-", today.Active: glyphToday + " Today", today.CarriedOver: glyphCarried + " Today (" + t.Today + " から持ち越し)", today.DoneToday: glyphDone + " Today (本日完了)"}[m.rules.Of(t, m.now())]
	status := t.Status
	field("状態", status, "")
	field("Today", state, th.Today)
	field("グループ", t.Project, th.Group)
	var tags []string
	for _, x := range t.Tags {
		tags = append(tags, "#"+x)
	}
	field("タグ", strings.Join(tags, " "), th.Tag)
	if d, ok := tw.ParseTime(t.Due); ok {
		field("期限", d.Local().Format("2006-01-02 15:04"), "")
	}
	if d, ok := tw.ParseTime(t.Scheduled); ok {
		field("予定", d.Local().Format("2006-01-02 15:04"), "")
	}
	field("UUID", t.UUID[:min(8, len(t.UUID))], th.Muted)
	out = append(out, "", " "+label.Render("メモ (jotnote)"))
	if t.Note == "" {
		out = append(out, "  "+label.Render("(なし — "+m.keyName("edit_note")+" で編集)"))
	} else {
		out = append(out, wrapLines(t.Note, w-3, func(s string) string { return "  " + val.Render(s) })...)
	}
	if len(t.Annotations) > 0 {
		out = append(out, "", " "+label.Render("注釈 (annotations)"))
		for _, a := range t.Annotations {
			out = append(out, wrapLines(oneLine(a.Description), w-5, func(s string) string { return "  - " + val.Render(s) })...)
		}
	}
	if len(out) <= h {
		m.detailOff = 0
		return out
	}
	if m.detailOff > len(out)-h+1 {
		m.detailOff = len(out) - h + 1
	}
	out = out[m.detailOff:]
	if len(out) > h {
		out = append(out[:h-1], label.Render(trunc(" -- "+m.keyName("down")+" で続き --", w)))
	}
	return out
}

func (m *Model) noteLines(w, h int) []string {
	t := m.findTask(m.noteUUID)
	name := ""
	if t != nil {
		name = oneLine(t.Description)
	}
	head := lipgloss.NewStyle().Foreground(m.theme.c(m.theme.Accent)).Bold(true).Render(trunc(" メモ編集: "+name, w))
	lines := []string{head}
	for _, l := range strings.Split(m.note.View(), "\n") {
		lines = append(lines, " "+ansi.Truncate(l, w-1, ""))
	}
	return lines
}

func (m *Model) urlLines(w, h int) []string {
	th := m.theme
	out := []string{lipgloss.NewStyle().Foreground(th.c(th.Accent)).Bold(true).Render(trunc(" 開くURLを選択 (http/https のみ開けます)", w))}
	for i, l := range m.urlList {
		sel := i == m.urlCursor
		cur := "  "
		if sel {
			cur = glyphCursor + " "
		}
		num := fmt.Sprintf("%d ", i+1)
		text := l.Raw
		color := th.FG
		if !l.Safe {
			text = "[ブロック] " + l.Raw + " — " + l.Reason
			color = th.Error
		}
		line := th.fg(th.Accent, sel).Render(cur) + th.fg(th.Muted, sel).Render(num) +
			th.fg(color, sel).Render(pad(oneLine(text), w-width(cur+num)))
		out = append(out, line)
	}
	return out
}

func (m *Model) helpLines(w, h int) []string {
	th := m.theme
	keySt := lipgloss.NewStyle().Foreground(th.c(th.Accent)).Bold(true)
	var all []string
	all = append(all, lipgloss.NewStyle().Bold(true).Render(trunc(" Jot キー一覧", w)))
	if m.cfg.Path != "" {
		all = append(all, lipgloss.NewStyle().Foreground(th.c(th.Muted)).Render(trunc(" 設定: "+m.cfg.Path, w)))
	} else {
		all = append(all, lipgloss.NewStyle().Foreground(th.c(th.Muted)).Render(trunc(" 設定: なし (既定値, 例: "+config.DefaultPath()+")", w)))
	}
	keyText := func(action string) string {
		var names []string
		for _, k := range m.cfg.KeysFor(action) {
			if k == " " {
				k = "space"
			}
			names = append(names, k)
		}
		return " " + strings.Join(names, " ")
	}
	kw := 0
	for _, a := range config.Actions {
		kw = max(kw, width(keyText(a.Name))+2)
	}
	kw = min(kw, max(8, w/2))
	for _, a := range config.Actions {
		ksText := pad(trunc(keyText(a.Name), kw-2), kw)
		all = append(all, keySt.Render(ksText)+trunc(a.Help, w-kw))
	}
	all = append(all, "", trunc(" 入力欄: enter 確定 / esc 取消。メモ: ctrl+s 保存 / esc 取消", w))
	if m.helpOff > len(all)-h {
		m.helpOff = max(0, len(all)-h)
	}
	return all[m.helpOff:]
}
