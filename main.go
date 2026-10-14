package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type view int

const (
	viewSnapshots view = iota
	viewTree
	viewRestorePrompt
	viewConfirmForget
	viewMessage
)

type crumb struct {
	path   string
	label  string
	nodes  []TreeNode // cached listing for this level, nil until loaded
	cursor int
	offset int
}

type model struct {
	view   view
	err    error
	msg    string
	height int // terminal height, from tea.WindowSizeMsg
	width  int

	snapshots  []Snapshot
	snapCursor int
	snapOffset int

	activeSnap  *Snapshot
	path        string // current dir path within the snapshot
	breadcrumbs []crumb
	nodes       []TreeNode
	treeCursor  int
	treeOffset  int
	treeLoading bool
	treeReqID   int    // incremented on every navigation; stale fetch results are discarded
	typeAhead   string // buffer of typed chars used to jump-select a node by name prefix

	restoreInput textinput.Model
	restoreNode  *TreeNode
}

// listWindow returns the visible slice bounds [start, end) for a list, and
// updates offset so the cursor always stays within the visible window.
func listWindow(cursor, offset, total, visible int) (start, end, newOffset int) {
	if visible < 1 {
		visible = 1
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}
	if offset < 0 {
		offset = 0
	}
	maxOffset := total - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	start = offset
	end = offset + visible
	if end > total {
		end = total
	}
	return start, end, offset
}

// reservedLines accounts for: title+breadcrumb line, esc-hint line, blank line,
// "N more above" indicator (reserved even when unused, to keep the budget stable),
// "N more below" indicator (same), blank line, help line.
const reservedLines = 7

func (m *model) visibleRows() int {
	v := m.height - reservedLines
	if v < 1 {
		v = 1
	}
	return v
}

func (m *model) clampSnapOffset() {
	_, _, offset := listWindow(m.snapCursor, m.snapOffset, len(m.snapshots), m.visibleRows())
	m.snapOffset = offset
}

func (m *model) clampTreeOffset() {
	_, _, offset := listWindow(m.treeCursor, m.treeOffset, len(m.nodes), m.visibleRows())
	m.treeOffset = offset
}

// typeAheadJump moves the cursor to the first node whose name has a
// case-insensitive prefix match on the current typeAhead buffer.
func (m *model) typeAheadJump() {
	if m.typeAhead == "" {
		return
	}
	q := strings.ToLower(m.typeAhead)
	for i, n := range m.nodes {
		if strings.HasPrefix(strings.ToLower(n.Name), q) {
			m.treeCursor = i
			m.clampTreeOffset()
			m.saveCursorToCrumb()
			return
		}
	}
	// no match — leave cursor where it is, keep buffer so backspace can correct it
}

func (m *model) saveCursorToCrumb() {
	if len(m.breadcrumbs) == 0 {
		return
	}
	last := len(m.breadcrumbs) - 1
	m.breadcrumbs[last].cursor = m.treeCursor
	m.breadcrumbs[last].offset = m.treeOffset
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	dirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	fileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	loadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

func initialModel() model {
	return model{view: viewSnapshots}
}

func (m model) Init() tea.Cmd {
	return loadSnapshotsCmd
}

type snapshotsLoadedMsg struct {
	snaps []Snapshot
	err   error
}

func loadSnapshotsCmd() tea.Msg {
	snaps, err := resticSnapshots()
	return snapshotsLoadedMsg{snaps: snaps, err: err}
}

type treeLoadedMsg struct {
	nodes []TreeNode
	err   error
	reqID int
}

func loadTreeCmd(snapID, path string, reqID int) tea.Cmd {
	return func() tea.Msg {
		nodes, err := resticLs(snapID, path)
		return treeLoadedMsg{nodes: nodes, err: err, reqID: reqID}
	}
}

type restoreDoneMsg struct {
	out string
	err error
}

func restoreCmd(snapID, includePath, target string) tea.Cmd {
	return func() tea.Msg {
		out, err := resticRestore(snapID, includePath, target)
		return restoreDoneMsg{out: out, err: err}
	}
}

type forgetDoneMsg struct {
	out string
	err error
}

func forgetCmd(snapID string) tea.Cmd {
	return func() tea.Msg {
		out, err := resticForget(snapID, false)
		return forgetDoneMsg{out: out, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case snapshotsLoadedMsg:
		m.err = msg.err
		m.snapshots = msg.snaps
		return m, nil
	case treeLoadedMsg:
		if msg.reqID != m.treeReqID {
			// stale result from a fetch we've since navigated away from; discard
			return m, nil
		}
		m.treeLoading = false
		m.err = msg.err
		m.nodes = msg.nodes
		if len(m.breadcrumbs) > 0 {
			m.breadcrumbs[len(m.breadcrumbs)-1].nodes = msg.nodes
		}
		return m, nil
	case restoreDoneMsg:
		m.view = viewMessage
		if msg.err != nil {
			m.msg = errStyle.Render("Restore failed:\n") + msg.out
		} else {
			m.msg = selectedStyle.Render("Restore complete.\n") + msg.out
		}
		return m, nil
	case forgetDoneMsg:
		m.view = viewMessage
		if msg.err != nil {
			m.msg = errStyle.Render("Forget failed:\n") + msg.out
		} else {
			m.msg = selectedStyle.Render("Snapshot forgotten.\n") + msg.out
		}
		return m, loadSnapshotsCmd
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewMessage:
		switch msg.String() {
		case "q", "esc", "enter":
			m.view = viewSnapshots
			return m, loadSnapshotsCmd
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil

	case viewSnapshots:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.snapCursor > 0 {
				m.snapCursor--
			}
			m.clampSnapOffset()
		case "down", "j":
			if m.snapCursor < len(m.snapshots)-1 {
				m.snapCursor++
			}
			m.clampSnapOffset()
		case "enter":
			if len(m.snapshots) == 0 {
				return m, nil
			}
			snap := m.snapshots[m.snapCursor]
			m.activeSnap = &snap
			m.path = snap.Paths[0]
			m.breadcrumbs = []crumb{{path: m.path, label: m.path}}
			m.view = viewTree
			m.nodes = nil
			m.treeCursor = 0
			m.treeOffset = 0
			m.typeAhead = ""
			m.treeLoading = true
			m.treeReqID++
			return m, loadTreeCmd(snap.ID, m.path, m.treeReqID)
		case "d":
			if len(m.snapshots) == 0 {
				return m, nil
			}
			m.view = viewConfirmForget
			return m, nil
		case "r":
			return m, loadSnapshotsCmd
		}
		return m, nil

	case viewConfirmForget:
		switch msg.String() {
		case "y":
			snap := m.snapshots[m.snapCursor]
			return m, forgetCmd(snap.ID)
		case "n", "esc":
			m.view = viewSnapshots
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil

	case viewTree:
		// esc and quit always work immediately, even mid-fetch; a stale
		// in-flight result is discarded via treeReqID when it eventually arrives.
		switch msg.String() {
		case "ctrl+c", "Q":
			return m, tea.Quit
		case "esc":
			m.treeReqID++ // invalidate any in-flight fetch for the level we're leaving
			m.treeLoading = false
			m.typeAhead = ""
			if len(m.breadcrumbs) > 1 {
				m.breadcrumbs = m.breadcrumbs[:len(m.breadcrumbs)-1]
				parent := m.breadcrumbs[len(m.breadcrumbs)-1]
				m.path = parent.path
				m.nodes = parent.nodes // already cached from when we first visited this level — no fetch needed
				m.treeCursor = parent.cursor
				m.treeOffset = parent.offset
				return m, nil
			}
			m.view = viewSnapshots
			return m, nil
		}

		if m.treeLoading {
			// ignore navigation/selection keys while a fetch is in flight
			return m, nil
		}

		switch msg.String() {
		case "up":
			if m.typeAhead != "" {
				return m, nil
			}
			if m.treeCursor > 0 {
				m.treeCursor--
			}
			m.clampTreeOffset()
			m.saveCursorToCrumb()
		case "down":
			if m.typeAhead != "" {
				return m, nil
			}
			if m.treeCursor < len(m.nodes)-1 {
				m.treeCursor++
			}
			m.clampTreeOffset()
			m.saveCursorToCrumb()
		case "enter":
			if len(m.nodes) == 0 {
				return m, nil
			}
			m.typeAhead = ""
			node := m.nodes[m.treeCursor]
			if node.Type == "dir" {
				m.path = node.Path
				m.breadcrumbs = append(m.breadcrumbs, crumb{path: node.Path, label: node.Name})
				m.nodes = nil
				m.treeCursor = 0
				m.treeOffset = 0
				m.treeLoading = true
				m.treeReqID++
				return m, loadTreeCmd(m.activeSnap.ID, node.Path, m.treeReqID)
			}
		case "R":
			if len(m.nodes) == 0 {
				return m, nil
			}
			m.typeAhead = ""
			node := m.nodes[m.treeCursor]
			m.restoreNode = &node
			home, _ := os.UserHomeDir()
			ti := textinput.New()
			ti.SetValue(filepath.Join(home, "restic-restores", node.Name))
			ti.CursorEnd()
			ti.Focus()
			ti.Width = 80
			m.restoreInput = ti
			m.view = viewRestorePrompt
			return m, nil
		case "backspace":
			if m.typeAhead != "" {
				r := []rune(m.typeAhead)
				m.typeAhead = string(r[:len(r)-1])
				m.typeAheadJump()
			}
		default:
			if len(msg.Runes) == 1 && msg.Runes[0] != ' ' {
				m.typeAhead += string(msg.Runes)
				m.typeAheadJump()
			}
		}
		return m, nil

	case viewRestorePrompt:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.view = viewTree
			return m, nil
		case "enter":
			target := m.restoreInput.Value()
			node := m.restoreNode
			m.view = viewMessage
			m.msg = "Restoring " + node.Path + " -> " + target + " ...\n"
			return m, restoreCmd(m.activeSnap.ID, node.Path, filepath.Dir(target))
		default:
			var cmd tea.Cmd
			m.restoreInput, cmd = m.restoreInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	if m.err != nil {
		b.WriteString(errStyle.Render("Error: "+m.err.Error()) + "\n\n")
	}

	visible := m.visibleRows()

	switch m.view {
	case viewSnapshots:
		b.WriteString(titleStyle.Render("Snapshots") + "\n\n")
		if len(m.snapshots) == 0 {
			b.WriteString(dimStyle.Render("Loading / no snapshots found...") + "\n")
		}
		start, end, _ := listWindow(m.snapCursor, m.snapOffset, len(m.snapshots), visible)
		if start > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more above", start)) + "\n")
		}
		for i := start; i < end; i++ {
			s := m.snapshots[i]
			line := fmt.Sprintf("%s  %s  %s", s.ShortID, s.Time.Format("2006-01-02 15:04:05"), strings.Join(s.Paths, ", "))
			if i == m.snapCursor {
				b.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
			}
		}
		if end < len(m.snapshots) {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(m.snapshots)-end)) + "\n")
		}
		b.WriteString("\n" + helpStyle.Render("↑/↓ move · enter browse · d forget · r refresh · q quit"))

	case viewConfirmForget:
		snap := m.snapshots[m.snapCursor]
		b.WriteString(errStyle.Render(fmt.Sprintf("Forget snapshot %s (%s)? [y/N]", snap.ShortID, snap.Time.Format("2006-01-02 15:04:05"))))

	case viewTree:
		crumbs := make([]string, len(m.breadcrumbs))
		for i, c := range m.breadcrumbs {
			crumbs[i] = c.label
		}
		escTarget := "snapshot list"
		if len(m.breadcrumbs) > 1 {
			escTarget = m.breadcrumbs[len(m.breadcrumbs)-2].label
		}
		b.WriteString(titleStyle.Render("Snapshot "+m.activeSnap.ShortID) + "  " + dimStyle.Render(strings.Join(crumbs, " / ")) + "\n")
		b.WriteString(dimStyle.Render("esc → "+escTarget))
		if m.typeAhead != "" {
			b.WriteString("   " + loadingStyle.Render("search: "+m.typeAhead+"█"))
		}
		b.WriteString("\n\n")
		if m.treeLoading {
			b.WriteString(loadingStyle.Render("⏳ loading...") + "\n")
		} else if len(m.nodes) == 0 {
			b.WriteString(dimStyle.Render("(empty)") + "\n")
		}
		tStart, tEnd, _ := listWindow(m.treeCursor, m.treeOffset, len(m.nodes), visible)
		if tStart > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more above", tStart)) + "\n")
		}
		for i := tStart; i < tEnd; i++ {
			n := m.nodes[i]
			icon := "📄"
			style := fileStyle
			if n.Type == "dir" {
				icon = "📁"
				style = dirStyle
			}
			line := fmt.Sprintf("%s %s", icon, n.Name)
			if i == m.treeCursor {
				b.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + style.Render(line) + "\n")
			}
		}
		if tEnd < len(m.nodes) {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(m.nodes)-tEnd)) + "\n")
		}
		if m.treeLoading {
			b.WriteString("\n" + helpStyle.Render("esc back (instant) · Q quit — navigation paused while loading"))
		} else {
			b.WriteString("\n" + helpStyle.Render("↑/↓ move · type to jump · enter open/select · R restore (shift+r) · esc back · Q quit (shift+q)"))
		}

	case viewRestorePrompt:
		b.WriteString(titleStyle.Render("Restore") + "\n\n")
		b.WriteString("Source: " + m.restoreNode.Path + "\n")
		b.WriteString("Target path:\n" + m.restoreInput.View() + "\n\n")
		b.WriteString(helpStyle.Render("enter confirm · esc cancel"))

	case viewMessage:
		b.WriteString(m.msg + "\n\n")
		b.WriteString(helpStyle.Render("enter/esc back to snapshots · q quit"))
	}

	return b.String()
}

func main() {
	if _, ok := os.LookupEnv("RESTIC_REPOSITORY"); !ok {
		fmt.Fprintln(os.Stderr, "RESTIC_REPOSITORY not set — source your restic env file first (e.g. `source ~/.restic-env`)")
		os.Exit(1)
	}
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
