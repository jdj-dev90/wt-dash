package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	markerWorking = "🤖"
	markerWaiting = "💬"
)

type mode int

const (
	modeNormal mode = iota
	modeNew
	modeConfirmRemove
)

type keyMap struct {
	Up, Down, Top, Bottom, Open, New, Remove, Diff, MR, Refresh, Help, Quit key.Binding
}

var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	Bottom:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open session")),
	New:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new worktree")),
	Remove:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "remove")),
	Diff:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff")),
	MR:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open MR")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// row points at one worktree: data[repo].Items[item].
type row struct{ repo, item int }

type model struct {
	repos    []string
	interval time.Duration
	notify   bool

	data       []repoData
	rows       []row
	cursor     int
	offset     int
	loading    bool
	lastUpdate time.Time
	markers    map[string]string // repo|branch -> last marker, for notifications
	focus      string            // repo|branch to select once it appears (new worktree)

	mode     mode
	input    textinput.Model
	status   string
	statusOK bool
	showHelp bool

	width, height int
}

type dataMsg []repoData
type tickMsg time.Time

func newModel(repos []string, interval time.Duration, notify bool) model {
	ti := textinput.New()
	ti.Placeholder = "PROJ-123"
	ti.CharLimit = 100
	ti.Prompt = ""
	ti.TextStyle = sFg
	ti.PlaceholderStyle = sDim
	return model{
		repos:    repos,
		interval: interval,
		notify:   notify,
		markers:  map[string]string{},
		input:    ti,
		loading:  true,
	}
}

func (m model) Init() tea.Cmd { return fetchAll(m.repos) }

func fetchAll(repos []string) tea.Cmd {
	return func() tea.Msg {
		out := make([]repoData, len(repos))
		var wg sync.WaitGroup
		for i, r := range repos {
			wg.Add(1)
			go func(i int, r string) {
				defer wg.Done()
				out[i] = wtList(context.Background(), r)
			}(i, r)
		}
		wg.Wait()
		return dataMsg(out)
	}
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) selected() (repoData, item, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return repoData{}, item{}, false
	}
	r := m.rows[m.cursor]
	rd := m.data[r.repo]
	return rd, rd.Items[r.item], true
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case dataMsg:
		m.applyData(msg)
		return m, tick(m.interval)

	case tickMsg:
		if m.loading {
			return m, tick(m.interval)
		}
		m.loading = true
		return m, fetchAll(m.repos)

	case execDoneMsg:
		if msg.err != nil {
			m.focus = "" // e.g. a failed create never appears
			m.setStatus(fmt.Sprintf("%s: %v", msg.what, msg.err), false)
		} else {
			m.setStatus(msg.what+" ✓", true)
		}
		m.loading = true
		return m, fetchAll(m.repos)

	case tea.KeyMsg:
		switch m.mode {
		case modeNew:
			return m.updateNew(msg)
		case modeConfirmRemove:
			return m.updateConfirm(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m *model) setStatus(s string, ok bool) { m.status, m.statusOK = s, ok }

// applyData swaps in fresh data, keeps the cursor on the same worktree, and
// fires a notification for every agent that went from working to waiting.
func (m *model) applyData(data []repoData) {
	prevKey := m.focus
	if rd, it, ok := m.selected(); ok && prevKey == "" {
		prevKey = rd.Path + "|" + it.branchName()
	}

	m.data = data
	m.rows = nil
	for ri, rd := range data {
		for ii, it := range rd.Items {
			k := rd.Path + "|" + it.branchName()
			if k == prevKey {
				m.cursor = len(m.rows)
				if k == m.focus {
					m.focus = ""
				}
			}
			if m.notify && m.markers[k] == markerWorking && it.Marker == markerWaiting {
				notify("Claude waiting", rd.Name+": "+it.branchName())
			}
			m.markers[k] = it.Marker
			m.rows = append(m.rows, row{ri, ii})
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = max(len(m.rows)-1, 0)
	}
	m.loading = false
	m.lastUpdate = time.Now()
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		m.cursor = max(m.cursor-1, 0)
	case key.Matches(msg, keys.Down):
		m.cursor = min(m.cursor+1, max(len(m.rows)-1, 0))
	case key.Matches(msg, keys.Top):
		m.cursor = 0
	case key.Matches(msg, keys.Bottom):
		m.cursor = max(len(m.rows)-1, 0)
	case key.Matches(msg, keys.Help):
		m.showHelp = !m.showHelp
	case key.Matches(msg, keys.Refresh):
		if !m.loading {
			m.loading = true
			return m, fetchAll(m.repos)
		}
	case key.Matches(msg, keys.Open):
		if _, it, ok := m.selected(); ok {
			m.setStatus("opening "+it.branchName()+"…", true)
			return m, openWorktree(it.path())
		}
	case key.Matches(msg, keys.New):
		if len(m.data) == 0 {
			return m, nil
		}
		m.mode = modeNew
		m.input.SetValue("")
		return m, m.input.Focus()
	case key.Matches(msg, keys.Remove):
		if _, it, ok := m.selected(); ok {
			if it.isMain() {
				m.setStatus("won't remove the main worktree", false)
				return m, nil
			}
			m.mode = modeConfirmRemove
		}
	case key.Matches(msg, keys.Diff):
		if rd, it, ok := m.selected(); ok {
			return m, showDiff(it.path(), rd.DefaultBranch)
		}
	case key.Matches(msg, keys.MR):
		if _, it, ok := m.selected(); ok {
			if u := it.prURL(); u != "" {
				return m, openURL(u)
			}
			m.setStatus("no MR for "+it.branchName(), false)
		}
	}
	return m, nil
}

// newTargetRepo is the repo a new worktree goes into: the selected row's repo.
func (m model) newTargetRepo() repoData {
	if rd, _, ok := m.selected(); ok {
		return rd
	}
	return m.data[0]
}

func (m model) updateNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		m.mode = modeNormal
		m.input.Blur()
		if name == "" {
			return m, nil
		}
		repo := m.newTargetRepo().Path
		m.focus = repo + "|" + name
		return m, newWorktree(repo, name)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeNormal
	if msg.String() != "y" {
		return m, nil
	}
	rd, it, ok := m.selected()
	if !ok {
		return m, nil
	}
	return m, removeWorktree(rd.Path, it.branchName())
}

// ---------- view ----------

type col struct {
	title string
	width int
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	w := m.width

	header := m.viewHeader(w)
	footer := m.viewFooter(w)
	bodyH := max(m.height-lipgloss.Height(header)-lipgloss.Height(footer), 1)

	lines, selLine := m.bodyLines(w)
	// keep the selected line visible
	off := m.offset
	if selLine >= 0 {
		if selLine < off {
			off = selLine
		}
		if selLine >= off+bodyH {
			off = selLine - bodyH + 1
		}
	}
	off = min(off, max(len(lines)-bodyH, 0))
	end := min(off+bodyH, len(lines))
	visible := lines[off:end]
	for len(visible) < bodyH {
		visible = append(visible, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(visible, "\n"), footer)
}

func (m model) viewHeader(w int) string {
	title := sTitle.Render(" wt-dash")
	state := sDim.Render("updated " + m.lastUpdate.Format("15:04:05"))
	if m.lastUpdate.IsZero() {
		state = sDim.Render("loading…")
	} else if m.loading {
		state = sDim.Render("refreshing…")
	}
	legend := sDim.Render(markerWorking + " working  " + markerWaiting + " waiting  ")
	gap := max(w-lipgloss.Width(title)-lipgloss.Width(legend)-lipgloss.Width(state)-1, 1)
	return title + strings.Repeat(" ", gap) + legend + state + " "
}

func columns(w int) []col {
	cols := []col{
		{"", 2}, // cursor
		{"", 3}, // agent marker
		{"Branch", 24},
		{"Status", 7},
		{"HEAD±", 12},
		{"main↕", 8},
		{"Remote", 7},
		{"MR/CI", 9},
		{"Age", 5},
	}
	used := 0
	for _, c := range cols {
		used += c.width + 1
	}
	cols = append(cols, col{"Message", max(w-used-1, 0)})
	return cols
}

func (m model) bodyLines(w int) ([]string, int) {
	cols := columns(w)
	var lines []string
	sel := -1
	rowIdx := 0

	for ri, rd := range m.data {
		if ri > 0 {
			lines = append(lines, "")
		}
		name := " " + sRepo.Render(rd.Name) + " "
		lines = append(lines, sRule.Render("──")+name+sRule.Render(strings.Repeat("─", max(w-lipgloss.Width(name)-2, 0))))
		if rd.Err != nil {
			lines = append(lines, "  "+sErr.Render(ansi.Truncate(rd.Err.Error(), w-4, "…")))
			continue
		}
		lines = append(lines, m.renderHeadRow(cols))
		for _, it := range rd.Items {
			isSel := rowIdx == m.cursor
			if isSel {
				sel = len(lines)
			}
			lines = append(lines, m.renderRow(cols, it, isSel, w))
			rowIdx++
		}
	}
	if len(m.data) == 0 && !m.loading {
		lines = append(lines, sDim.Render("  no repos"))
	}
	return lines, sel
}

func cell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, "…")
	return s + strings.Repeat(" ", max(width-lipgloss.Width(s), 0))
}

func (m model) renderHeadRow(cols []col) string {
	var b strings.Builder
	for _, c := range cols {
		b.WriteString(cell(sColHead.Render(c.title), c.width) + " ")
	}
	return b.String()
}

func (m model) renderRow(cols []col, it item, selected bool, w int) string {
	cursor := ""
	if selected {
		cursor = sCursor.Render("▌")
	}
	branch := sBranch.Render(it.branchName())
	if it.isMain() {
		branch = sMainBr.Render(it.branchName())
	}

	cells := []string{
		cursor,
		it.Marker,
		branch,
		sStatus.Render(stripMarker(it.Display.Symbols, it.Marker)),
		fmtHeadDiff(it),
		fmtAheadBehind(it),
		fmtUpstream(it),
		fmtCI(it),
		sDim.Render(fmtAge(it)),
		sDim.Render(it.subject()),
	}
	var b strings.Builder
	for i, c := range cols {
		b.WriteString(cell(cells[i], c.width) + " ")
	}
	line := b.String()
	if selected {
		// re-apply the selection background after every inner reset
		line = selBg + strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+selBg)
		line = cell(line, w) + "\x1b[0m"
	}
	return line
}

// stripMarker drops the marker emoji wt appends to display.symbols; it has its own column.
func stripMarker(symbols, marker string) string {
	if marker != "" {
		symbols = strings.TrimSuffix(symbols, marker)
	}
	return symbols
}

func fmtHeadDiff(it item) string {
	if it.Worktree == nil || it.Worktree.Changes.Diff == nil {
		return ""
	}
	d := it.Worktree.Changes.Diff
	var parts []string
	if d.Added > 0 {
		parts = append(parts, sAdd.Render("+"+short(d.Added)))
	}
	if d.Deleted > 0 {
		parts = append(parts, sDel.Render("-"+short(d.Deleted)))
	}
	return strings.Join(parts, " ")
}

func fmtAheadBehind(it item) string {
	if it.DefaultBranch == nil {
		return ""
	}
	var parts []string
	if it.DefaultBranch.Ahead > 0 {
		parts = append(parts, sAhead.Render(fmt.Sprintf("↑%d", it.DefaultBranch.Ahead)))
	}
	if it.DefaultBranch.Behind > 0 {
		parts = append(parts, sBehind.Render(fmt.Sprintf("↓%d", it.DefaultBranch.Behind)))
	}
	return strings.Join(parts, " ")
}

func fmtUpstream(it item) string {
	if it.Upstream == nil {
		return ""
	}
	var parts []string
	if it.Upstream.Ahead > 0 {
		parts = append(parts, sAhead.Render(fmt.Sprintf("⇡%d", it.Upstream.Ahead)))
	}
	if it.Upstream.Behind > 0 {
		parts = append(parts, sBehind.Render(fmt.Sprintf("⇣%d", it.Upstream.Behind)))
	}
	return strings.Join(parts, " ")
}

func fmtCI(it item) string {
	label := ""
	if it.PR != nil && it.PR.Number != nil {
		label = fmt.Sprintf("!%d", *it.PR.Number)
	} else if it.Checks != nil {
		label = "#"
	}
	if label == "" {
		return ""
	}
	style, mark := sCINone, ""
	if it.Checks != nil && it.Checks.Status != nil {
		switch *it.Checks.Status {
		case "passed":
			style, mark = sCIPass, " ✓"
		case "running":
			style, mark = sCIRun, " ●"
		case "failed":
			style, mark = sCIFail, " ✗"
		}
	}
	out := style.Render(label + mark)
	if it.PR != nil && it.PR.Review == "approved" {
		out += sReview.Render(" ✔")
	}
	return out
}

func fmtAge(it item) string {
	t, ok := it.committedAt()
	if !ok {
		return ""
	}
	return humanAge(time.Since(t))
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw", int(d.Hours()/24/7))
	}
}

func short(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dK", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

func (m model) viewFooter(w int) string {
	var top string
	switch m.mode {
	case modeNew:
		top = sPrompt.Render(" new worktree in "+m.newTargetRepo().Name+" › ") + m.input.View() + sDim.Render("   enter create · esc cancel")
	case modeConfirmRemove:
		_, it, _ := m.selected()
		top = sErr.Render(" remove "+it.branchName()+" (worktree + branch)? ") + sDim.Render("y / n")
	default:
		if m.status != "" {
			st := sErr
			if m.statusOK {
				st = sOK
			}
			top = " " + st.Render(ansi.Truncate(m.status, w-2, "…"))
		}
	}

	bindings := []key.Binding{keys.Open, keys.New, keys.Remove, keys.Diff, keys.MR, keys.Refresh, keys.Help, keys.Quit}
	if m.showHelp {
		bindings = []key.Binding{keys.Up, keys.Down, keys.Top, keys.Bottom, keys.Open, keys.New, keys.Remove, keys.Diff, keys.MR, keys.Refresh, keys.Help, keys.Quit}
	}
	var parts []string
	for _, b := range bindings {
		h := b.Help()
		parts = append(parts, sKey.Render(h.Key)+" "+sDim.Render(h.Desc))
	}
	help := " " + strings.Join(parts, sDim.Render(" · "))
	if lipgloss.Width(help) > w {
		help = ansi.Truncate(help, w, "…")
	}
	return lipgloss.JoinVertical(lipgloss.Left, sRule.Render(strings.Repeat("─", w)), top, help)
}
