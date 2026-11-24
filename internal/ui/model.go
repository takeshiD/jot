// Package ui is Jot's Bubble Tea interface.
package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tkcd/jot/internal/config"
	"github.com/tkcd/jot/internal/today"
	"github.com/tkcd/jot/internal/tw"
	"github.com/tkcd/jot/internal/urls"
)

type view int

const (
	viewToday view = iota
	viewAll
)

type mode int

const (
	modeList mode = iota
	modeInput
	modeNote
	modeHelp
	modeDetail
	modeURL
	modeConfirm
)

type inputKind int

const (
	inAdd inputKind = iota
	inTitle
	inTags
	inProject
)

// Backend is the Taskwarrior API the UI needs (implemented by *tw.Client).
type Backend interface {
	Export() ([]tw.Task, error)
	Add(title string, mods ...string) (string, error)
	Modify(uuid string, args ...string) error
	SetText(uuid, field, value string) error
	Done(uuid string) error
	Undone(uuid string) error
	Delete(uuid string) error
}

// Options configure a Model.
type Options struct {
	Config   *config.Config
	Backend  Backend
	Clock    func() time.Time
	FakeTime bool // clock is overridden: pass end:<now> when completing
	Opener   func(argv []string, url string) ([]string, error)
}

type row struct {
	header string // non-empty: group header row
	count  int
	task   *tw.Task
}

// Model is the Bubble Tea model.
type Model struct {
	cfg      *config.Config
	be       Backend
	clock    func() time.Time
	fake     bool
	opener   func([]string, string) ([]string, error)
	rules    today.Rules
	theme    Theme
	width    int
	height   int
	view     view
	mode     mode
	grouped  bool
	showDone bool

	tasks   []tw.Task
	rows    []row
	cursor  int
	offset  int
	selUUID string
	day     string

	status    string
	statusErr bool

	input       textinput.Model
	inKind      inputKind
	inUUID      string
	note        textarea.Model
	noteUUID    string
	helpOff     int
	detailUUID  string
	detailOff   int
	quitting    bool
	urlList     []urls.Link
	urlCursor   int
	confirmUUID string
	prevMode    mode
}

// New creates the model and performs the initial load.
func New(o Options) *Model {
	if o.Clock == nil {
		o.Clock = time.Now
	}
	if o.Opener == nil {
		o.Opener = urls.Open
	}
	th, ok := Themes[o.Config.Theme]
	if !ok {
		th = Themes["dark"]
	}
	m := &Model{
		cfg: o.Config, be: o.Backend, clock: o.Clock, fake: o.FakeTime, opener: o.Opener,
		rules:   today.Rules{DayStartHour: o.Config.DayStartHour, CarryOver: o.Config.CarryOverEnabled()},
		theme:   th,
		width:   80,
		height:  24,
		grouped: o.Config.GroupBy != "none",
	}
	m.day = m.rules.Day(m.clock())
	m.reload("")
	return m
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return tick() }

func (m *Model) setStatus(s string) { m.status, m.statusErr = s, false }
func (m *Model) setError(err error) { m.status, m.statusErr = "エラー: "+err.Error(), true }

// reload re-reads all tasks via `task export` and rebuilds rows.
func (m *Model) reload(selectUUID string) bool {
	if selectUUID == "" {
		selectUUID = m.selUUID
	}
	ts, err := m.be.Export()
	if err != nil {
		m.setError(err)
		return false
	}
	m.tasks = ts
	m.rebuild(selectUUID)
	return true
}

func (m *Model) now() time.Time { return m.clock() }

func (m *Model) todayList() []tw.Task { return m.rules.List(m.tasks, m.now()) }

func (m *Model) countAll() int {
	n := 0
	for _, t := range m.tasks {
		if t.IsPending() {
			n++
		}
	}
	return n
}

// rebuild computes visible rows for the current view.
func (m *Model) rebuild(selectUUID string) {
	var rows []row
	if m.view == viewToday {
		list := m.todayList()
		for i := range list {
			t := list[i]
			rows = append(rows, row{task: &t})
		}
	} else {
		var list []tw.Task
		for _, t := range m.tasks {
			if t.IsPending() || (m.showDone && t.IsCompleted()) {
				list = append(list, t)
			}
		}
		sort.SliceStable(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if a.IsPending() != b.IsPending() {
				return a.IsPending()
			}
			if a.IsPending() {
				if a.Urgency != b.Urgency {
					return a.Urgency > b.Urgency
				}
				return a.Entry < b.Entry
			}
			return a.End > b.End
		})
		if m.grouped {
			groups := map[string][]tw.Task{}
			var names []string
			for _, t := range list {
				if _, ok := groups[t.Project]; !ok {
					names = append(names, t.Project)
				}
				groups[t.Project] = append(groups[t.Project], t)
			}
			sort.Slice(names, func(i, j int) bool {
				if (names[i] == "") != (names[j] == "") {
					return names[j] == ""
				}
				return names[i] < names[j]
			})
			for _, n := range names {
				h := n
				if h == "" {
					h = "(グループなし)"
				}
				rows = append(rows, row{header: h, count: len(groups[n])})
				for i := range groups[n] {
					t := groups[n][i]
					rows = append(rows, row{task: &t})
				}
			}
		} else {
			for i := range list {
				t := list[i]
				rows = append(rows, row{task: &t})
			}
		}
	}
	oldCursor := m.cursor
	m.rows = rows
	m.cursor = -1
	if selectUUID != "" {
		for i, r := range rows {
			if r.task != nil && r.task.UUID == selectUUID {
				m.cursor = i
				break
			}
		}
	}
	if m.cursor < 0 {
		m.cursor = oldCursor
		if m.cursor >= len(rows) {
			m.cursor = len(rows) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.fixCursor(1)
	}
	m.syncSel()
}

// fixCursor moves the cursor onto a task row (searching in dir first).
func (m *Model) fixCursor(dir int) {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	for _, d := range []int{dir, -dir} {
		for i := m.cursor; i >= 0 && i < len(m.rows); i += d {
			if m.rows[i].task != nil {
				m.cursor = i
				return
			}
		}
	}
}

func (m *Model) syncSel() {
	if t := m.selected(); t != nil {
		m.selUUID = t.UUID
	}
}

func (m *Model) selected() *tw.Task {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].task
	}
	return nil
}

func (m *Model) findTask(uuid string) *tw.Task {
	for i := range m.tasks {
		if m.tasks[i].UUID == uuid {
			return &m.tasks[i]
		}
	}
	return nil
}

func (m *Model) moveCursor(d int) {
	if len(m.rows) == 0 {
		return
	}
	i := m.cursor + d
	for i >= 0 && i < len(m.rows) && m.rows[i].task == nil {
		i += d
	}
	if i >= 0 && i < len(m.rows) {
		m.cursor = i
	}
	m.syncSel()
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeEditors()
		return m, nil
	case tickMsg:
		if d := m.rules.Day(m.now()); d != m.day {
			m.day = d
			m.rebuild("")
			m.setStatus("日付が変わりました: " + d)
		}
		return m, tick()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) action(k tea.KeyMsg) string {
	return m.cfg.KeyMap[k.String()]
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Fast typing/key repeat can deliver several runes in one message; outside
	// text editors treat them as individual key presses.
	if k.Type == tea.KeyRunes && len(k.Runes) > 1 && !k.Paste && m.mode != modeInput && m.mode != modeNote {
		var cmds []tea.Cmd
		for _, r := range k.Runes {
			_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: k.Alt})
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}
	switch m.mode {
	case modeInput:
		return m.keyInput(k)
	case modeNote:
		return m.keyNote(k)
	case modeHelp:
		switch {
		case k.String() == "esc" || m.action(k) == "help" || m.action(k) == "quit":
			m.mode = modeList
		case m.action(k) == "down":
			m.helpOff++
		case m.action(k) == "up":
			if m.helpOff > 0 {
				m.helpOff--
			}
		}
		return m, nil
	case modeURL:
		return m.keyURL(k)
	case modeConfirm:
		switch k.String() {
		case "y", "Y":
			m.mode = modeList
			m.doDelete(m.confirmUUID)
		default:
			m.mode = m.prevMode
			m.setStatus("削除を取り消しました")
		}
		return m, nil
	case modeDetail:
		if k.String() == "esc" || m.action(k) == "quit" || k.String() == "h" || k.String() == "left" {
			m.mode = modeList
			m.status = ""
			return m, nil
		}
		if k.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		switch m.action(k) {
		case "down":
			m.detailOff++
			return m, nil
		case "up":
			if m.detailOff > 0 {
				m.detailOff--
			}
			return m, nil
		case "top", "bottom", "switch_view", "view_today", "view_all", "detail":
			return m, nil
		}
	}
	act := m.action(k)
	t := m.selected()
	if m.mode == modeDetail {
		t = m.findTask(m.detailUUID)
	}
	switch act {
	case "quit":
		m.quitting = true
		return m, tea.Quit
	case "help":
		m.mode, m.helpOff = modeHelp, 0
	case "up":
		m.moveCursor(-1)
	case "down":
		m.moveCursor(1)
	case "top":
		m.cursor = 0
		m.fixCursor(1)
		m.syncSel()
	case "bottom":
		m.cursor = len(m.rows) - 1
		m.fixCursor(-1)
		m.syncSel()
	case "switch_view":
		if m.view == viewToday {
			m.setView(viewAll)
		} else {
			m.setView(viewToday)
		}
	case "view_today":
		m.setView(viewToday)
	case "view_all":
		m.setView(viewAll)
	case "toggle_group":
		m.grouped = !m.grouped
		if m.view == viewToday {
			m.setView(viewAll)
		}
		m.rebuild("")
		if m.grouped {
			m.setStatus("グループ表示: project")
		} else {
			m.setStatus("グループ表示: なし")
		}
	case "toggle_completed":
		m.showDone = !m.showDone
		if m.view == viewToday {
			m.setView(viewAll)
		}
		m.rebuild("")
		if m.showDone {
			m.setStatus("完了済みタスクを表示")
		} else {
			m.setStatus("完了済みタスクを非表示")
		}
	case "refresh":
		if m.reload("") {
			m.setStatus(fmt.Sprintf("再読込しました (%d件)", len(m.tasks)))
		}
	case "add":
		m.startInput(inAdd, "", "")
	case "toggle_today":
		if t != nil {
			m.toggleToday(*t)
		}
	case "move_up", "move_down":
		if m.view != viewToday || m.mode == modeDetail {
			m.setStatus("並べ替えはToday画面で行えます (tab で切替)")
		} else if t != nil {
			d := -1
			if act == "move_down" {
				d = 1
			}
			m.move(*t, d)
		}
	case "edit_title":
		if t != nil {
			m.startInput(inTitle, t.UUID, t.Description)
		}
	case "edit_tags":
		if t != nil {
			m.startInput(inTags, t.UUID, strings.Join(t.Tags, " "))
		}
	case "edit_project":
		if t != nil {
			m.startInput(inProject, t.UUID, t.Project)
		}
	case "edit_note":
		if t != nil {
			m.startNote(*t)
		}
	case "toggle_done":
		if t != nil {
			m.toggleDone(*t)
		}
	case "delete":
		if t != nil {
			m.prevMode = m.mode
			m.mode, m.confirmUUID = modeConfirm, t.UUID
			m.status, m.statusErr = fmt.Sprintf("「%s」を削除しますか? (y: 削除 / その他: 取消)", t.Description), true
		}
	case "open_url":
		if t != nil {
			m.startURL(*t)
		}
	case "detail":
		if t != nil {
			m.mode, m.detailUUID, m.status, m.detailOff = modeDetail, t.UUID, "", 0
		}
	}
	return m, nil
}

func (m *Model) setView(v view) {
	if m.mode == modeDetail {
		m.mode = modeList
	}
	m.view = v
	m.rebuild(m.selUUID)
}

// --- operations (each writes through `task`, then reloads via export) ---

func (m *Model) after(err error, okMsg, sel string) {
	if err != nil {
		m.setError(err)
		m.reload(sel)
		return
	}
	if m.reload(sel) {
		m.setStatus(okMsg)
	}
}

func (m *Model) toggleToday(t tw.Task) {
	st := m.rules.Of(t, m.now())
	if st != today.NotToday {
		err := m.be.Modify(t.UUID, tw.UDAToday+":", tw.UDAOrder+":")
		m.after(err, "Todayから外しました: "+t.Description, "")
		return
	}
	if !t.IsPending() {
		m.setStatus("完了済みのタスクはTodayに追加できません (x で完了取消)")
		return
	}
	order := today.NextOrder(m.todayList())
	err := m.be.Modify(t.UUID, tw.UDAToday+":"+m.day, fmt.Sprintf("%s:%d", tw.UDAOrder, order))
	m.after(err, "Todayに追加しました: "+t.Description, t.UUID)
}

func (m *Model) move(t tw.Task, d int) {
	list := m.todayList()
	idx := -1
	for i, x := range list {
		if x.UUID == t.UUID {
			idx = i
		}
	}
	changes, _, ok := today.Move(list, idx, d)
	if !ok {
		return
	}
	for _, c := range changes {
		if err := m.be.Modify(c.UUID, fmt.Sprintf("%s:%d", tw.UDAOrder, c.Order)); err != nil {
			m.after(err, "", t.UUID)
			return
		}
	}
	m.after(nil, "並べ替えました", t.UUID)
}

func (m *Model) toggleDone(t tw.Task) {
	if t.IsCompleted() {
		err := m.be.Undone(t.UUID)
		m.after(err, "完了を取り消しました: "+t.Description, t.UUID)
		return
	}
	var err error
	if m.fake {
		err = m.be.Modify(t.UUID, "status:completed", "end:"+m.now().UTC().Format("20060102T150405Z"))
	} else {
		err = m.be.Done(t.UUID)
	}
	m.after(err, "完了しました: "+t.Description+" (x で取消)", t.UUID)
}

func (m *Model) doDelete(uuid string) {
	t := m.findTask(uuid)
	name := uuid
	if t != nil {
		name = t.Description
	}
	err := m.be.Delete(uuid)
	m.after(err, "削除しました: "+name, "")
}

// --- single-line input ---

func (m *Model) startInput(kind inputKind, uuid, value string) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 0
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.SetValue(value)
	ti.CursorEnd()
	ti.Focus()
	ti.Width = max(10, m.width-len([]rune(inputLabel(kind)))*2-2)
	m.input, m.inKind, m.inUUID = ti, kind, uuid
	m.prevMode = m.mode
	m.mode = modeInput
	m.status = ""
}

func inputLabel(k inputKind) string {
	switch k {
	case inAdd:
		return "新規タスク: "
	case inTitle:
		return "タイトル: "
	case inTags:
		return "タグ(空白区切り): "
	default:
		return "グループ(project): "
	}
}

func (m *Model) keyInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.Type == tea.KeyRunes {
		var cmds []tea.Cmd
		for _, r := range splitRunes(k) {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(r)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}
	switch k.String() {
	case "esc", "ctrl+c":
		m.mode = m.prevMode
		m.setStatus("取り消しました")
		return m, nil
	case "enter":
		m.mode = m.prevMode
		m.commitInput(m.input.Value())
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	return m, cmd
}

func (m *Model) commitInput(v string) {
	switch m.inKind {
	case inAdd:
		title := tw.CleanTitle(v)
		if title == "" {
			m.setStatus("空のタイトルは追加しません")
			return
		}
		var mods []string
		if m.view == viewToday {
			mods = append(mods, tw.UDAToday+":"+m.day, fmt.Sprintf("%s:%d", tw.UDAOrder, today.NextOrder(m.todayList())))
		}
		u, err := m.be.Add(title, mods...)
		where := ""
		if m.view == viewToday {
			where = " (Today)"
		}
		m.after(err, "追加しました"+where+": "+title, u)
	case inTitle:
		title := tw.CleanTitle(v)
		if title == "" {
			m.setStatus("タイトルは空にできません")
			return
		}
		m.after(m.be.SetText(m.inUUID, "description", title), "タイトルを変更しました", m.inUUID)
	case inTags:
		t := m.findTask(m.inUUID)
		if t == nil {
			return
		}
		want := map[string]bool{}
		var args []string
		for _, f := range strings.Fields(v) {
			f = strings.TrimPrefix(f, "+")
			f = strings.TrimPrefix(f, "#")
			if !tw.ValidTag(f) {
				m.setError(fmt.Errorf("使えないタグ名: %q (空白・: ( ) 引用符は不可)", f))
				return
			}
			if !want[f] && !t.HasTag(f) {
				args = append(args, "+"+f)
			}
			want[f] = true
		}
		for _, old := range t.Tags {
			if !want[old] {
				args = append(args, "-"+old)
			}
		}
		if len(args) == 0 {
			m.setStatus("タグは変更されていません")
			return
		}
		m.after(m.be.Modify(m.inUUID, args...), "タグを更新しました: "+strings.Join(args, " "), m.inUUID)
	case inProject:
		p := strings.TrimSpace(v)
		if !tw.ValidProject(p) {
			m.setError(fmt.Errorf("使えないグループ名: %q (空白・引用符・括弧は不可)", p))
			return
		}
		m.after(m.be.Modify(m.inUUID, "project:"+p), "グループを変更しました", m.inUUID)
	}
}

// --- multi-line note ---

func (m *Model) startNote(t tw.Task) {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.Cursor.SetMode(cursor.CursorStatic)
	ta.SetValue(t.Note)
	ta.Focus()
	m.note, m.noteUUID = ta, t.UUID
	m.prevMode = m.mode
	m.mode = modeNote
	m.resizeEditors()
}

func (m *Model) resizeEditors() {
	if m.mode == modeNote {
		m.note.SetWidth(max(10, m.width-2))
		m.note.SetHeight(max(3, m.height-4))
	}
	if m.mode == modeInput {
		m.input.Width = max(10, m.width-len([]rune(inputLabel(m.inKind)))*2-2)
	}
}

func (m *Model) keyNote(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.Type == tea.KeyRunes {
		var cmds []tea.Cmd
		for _, r := range splitRunes(k) {
			var cmd tea.Cmd
			m.note, cmd = m.note.Update(r)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}
	switch k.String() {
	case "esc":
		m.mode = m.prevMode
		m.setStatus("メモの編集を取り消しました")
		return m, nil
	case "ctrl+s":
		m.mode = m.prevMode
		v := strings.TrimRight(strings.ReplaceAll(m.note.Value(), "\r\n", "\n"), "\n ")
		m.after(m.be.SetText(m.noteUUID, tw.UDANote, v), "メモを保存しました", m.noteUUID)
		return m, nil
	}
	var cmd tea.Cmd
	m.note, cmd = m.note.Update(k)
	return m, cmd
}

// --- URL picker ---

func taskLinks(t tw.Task) []urls.Link {
	text := t.Note
	for _, a := range t.Annotations {
		text += "\n" + a.Description
	}
	return urls.Extract(text)
}

func (m *Model) startURL(t tw.Task) {
	links := taskLinks(t)
	if len(links) == 0 {
		m.setStatus("このタスクのメモにURLはありません (n でメモ編集)")
		return
	}
	m.urlList = links
	m.urlCursor = 0
	for i, l := range links {
		if l.Safe {
			m.urlCursor = i
			break
		}
	}
	m.prevMode = m.mode
	m.mode = modeURL
	m.status = ""
}

func (m *Model) keyURL(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch {
	case s == "esc" || m.action(k) == "quit":
		m.mode = m.prevMode
		return m, nil
	case m.action(k) == "up":
		if m.urlCursor > 0 {
			m.urlCursor--
		}
	case m.action(k) == "down":
		if m.urlCursor < len(m.urlList)-1 {
			m.urlCursor++
		}
	case s == "enter" || m.action(k) == "open_url":
		m.openURL(m.urlCursor)
	case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
		m.openURL(int(s[0] - '1'))
	}
	return m, nil
}

func (m *Model) openURL(i int) {
	if i < 0 || i >= len(m.urlList) {
		return
	}
	l := m.urlList[i]
	if !l.Safe {
		m.status, m.statusErr = "開きません (安全でないURL): "+l.Reason, true
		return
	}
	m.mode = m.prevMode
	if _, err := m.opener(m.cfg.Browser.Command, l.Raw); err != nil {
		m.setError(err)
		return
	}
	m.setStatus("開きました: " + l.Raw)
}

// splitRunes turns a KeyRunes message into one message per rune, so that
// typed text such as "home", "enter" or "esc" arriving in a single read is
// always inserted as characters and never matched as a key name (by Jot or
// by the bubbles key bindings, which compare msg.String()). Pasted text
// keeps its Paste flag.
func splitRunes(k tea.KeyMsg) []tea.KeyMsg {
	if len(k.Runes) <= 1 || k.Paste {
		return []tea.KeyMsg{k}
	}
	out := make([]tea.KeyMsg, 0, len(k.Runes))
	for _, r := range k.Runes {
		out = append(out, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: k.Alt, Paste: k.Paste})
	}
	return out
}
