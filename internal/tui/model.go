// Package tui implements koboctl's interactive Bubble Tea interface: a live
// device dashboard, a full manifest editor, and action runners (provision,
// install, harden, backup) with streamed output.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	initcmd "github.com/seandheath/koboctl/internal/init"
	"github.com/seandheath/koboctl/internal/manifest"
	"github.com/seandheath/koboctl/internal/mstore"
	"github.com/seandheath/koboctl/internal/plugins"
)

type focusArea int

const (
	focusTree focusArea = iota
	focusLog
)

type modalKind int

const (
	modalNone modalKind = iota
	modalConfirm
	modalDiff
	modalPlugins
	modalInstall
	modalMessage
)

// action names (also shown in the log as "$ <name>").
const (
	actProvisionDry = "provision (dry-run)"
	actProvision    = "provision"
	actHardenDry    = "harden (dry-run)"
	actHarden       = "harden"
)

// model is the root Bubble Tea model. It is used as a pointer so background
// action goroutines can Send into it via prog.
type model struct {
	prog    *tea.Program
	actions Actions

	manifestPath string // where the manifest was loaded from (device or host)
	hostPath     string // host fallback path (from --manifest)
	mountPath    string // --mount override ("" = auto)

	m        *manifest.Manifest
	original manifest.Manifest
	roots    []*node
	flat     []*node
	cursor   int
	treeTop  int // scroll offset in the tree pane

	st     styles
	width  int
	height int

	focus focusArea
	log   viewport.Model
	lines []string
	input textinput.Model
	edit  *node // field being text-edited (nil otherwise)

	busy bool

	// live device status
	status deviceStatusMsg

	// modal state
	modal        modalKind
	modalTitle   string
	modalLines   []string
	modalCursor  int
	choices      []string
	pluginChecks map[string]bool
	confirmFn    func() tea.Cmd
}

// newModel builds the initial model. Device-primary: the manifest is resolved
// from the connected Kobo's .adds/koboctl/koboctl.toml when present, else the
// host path, else SecureDefaults() (first-run).
func newModel(manifestPath, mountPath string, actions Actions) *model {
	r, err := mstore.Load(manifestPath, mountPath)
	if err != nil {
		def := initcmd.SecureDefaults()
		r = &mstore.Resolved{Manifest: &def, Path: manifestPath, Device: mstore.Detect(mountPath)}
	}
	m := r.Manifest

	ti := textinput.New()
	ti.Prompt = "› "

	mdl := &model{
		actions:      actions,
		manifestPath: r.Path,
		hostPath:     manifestPath,
		mountPath:    mountPath,
		m:            m,
		original:     *m,
		st:           newStyles(),
		input:        ti,
		log:          viewport.New(0, 0),
		pluginChecks: map[string]bool{},
		status:       deviceStatusMsg{di: r.Device}, // seed the panel; poll fills components
	}
	mdl.rebuildTree()
	return mdl
}

func (m *model) rebuildTree() {
	m.roots = buildTree(m.m)
	m.flat = flatten(m.roots)
	if m.cursor >= len(m.flat) {
		m.cursor = len(m.flat) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(pollDeviceCmd(m.mountPath, m.m), tickCmd())
}

// dirty reports whether the live manifest differs from the last saved copy.
func (m *model) dirty() bool {
	cur, _ := initcmd.Render(*m.m)
	orig, _ := initcmd.Render(m.original)
	return cur != orig
}

func (m *model) mount() string {
	if m.status.di != nil {
		return m.status.di.MountPoint
	}
	return m.mountPath
}

func (m *model) appendLog(line string) {
	m.lines = append(m.lines, line)
	m.log.SetContent(strings.Join(m.lines, "\n"))
	m.log.GotoBottom()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tickMsg:
		return m, tea.Batch(pollDeviceCmd(m.mountPath, m.m), tickCmd())

	case deviceStatusMsg:
		m.status = msg
		return m, nil

	case logLineMsg:
		m.appendLog(string(msg))
		return m, nil

	case actionDoneMsg:
		return m, m.onActionDone(msg)

	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// layout sizes the log viewport based on the current window.
func (m *model) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	logH := m.height / 3
	if logH < 5 {
		logH = 5
	}
	m.log.Width = m.width - 4
	m.log.Height = logH
}

func (m *model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 1. Text editing takes precedence.
	if m.edit != nil {
		switch msg.String() {
		case "enter":
			m.commitEdit()
			return m, nil
		case "esc":
			m.edit = nil
			m.input.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	// 2. Modal handling.
	if m.modal != modalNone {
		return m.onModalKey(msg)
	}

	// 3. Global keys.
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case "tab":
		if m.focus == focusTree {
			m.focus = focusLog
		} else {
			m.focus = focusTree
		}
		return m, nil
	case "ctrl+r", "f5":
		return m, pollDeviceCmd(m.mountPath, m.m)
	}

	if m.focus == focusLog {
		var cmd tea.Cmd
		m.log, cmd = m.log.Update(msg)
		return m, cmd
	}

	// Tree-focused keys.
	switch msg.String() {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case " ", "enter":
		return m, m.activate()
	case "left", "h":
		m.onLeftRight(-1)
	case "right", "l":
		m.onLeftRight(1)
	case "P":
		return m, m.guardedAction(actProvisionDry, func() error { return m.actions.Provision(m.mountPath, m.m, true) })
	case "H":
		return m, m.guardedAction(actHardenDry, func() error { return m.actions.Harden(m.mount(), m.m.Hardening, true, false) })
	case "I":
		m.openInstallMenu()
	case "B":
		return m, m.confirmBackup()
	case "S":
		m.openSaveDiff()
	}
	return m, nil
}

func (m *model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.flat) {
		m.cursor = len(m.flat) - 1
	}
}

// activate handles space/enter on the current node.
func (m *model) activate() tea.Cmd {
	if len(m.flat) == 0 {
		return nil
	}
	n := m.flat[m.cursor]
	switch n.kind {
	case kindGroup:
		n.expanded = !n.expanded
		m.flat = flatten(m.roots)
		m.moveCursor(0)
	case kindBool:
		n.toggle()
	case kindEnum:
		n.cycle(1)
	case kindString:
		m.startEdit(n, n.value())
	case kindStringList:
		m.startEdit(n, strings.Join(n.getList(), ", "))
	case kindPlugins:
		m.openPlugins(n)
	}
	return nil
}

// onLeftRight cycles enums or collapses/expands groups.
func (m *model) onLeftRight(dir int) {
	if len(m.flat) == 0 {
		return
	}
	n := m.flat[m.cursor]
	switch n.kind {
	case kindEnum:
		n.cycle(dir)
	case kindGroup:
		n.expanded = dir > 0
		m.flat = flatten(m.roots)
		m.moveCursor(0)
	}
}

func (m *model) startEdit(n *node, cur string) {
	m.edit = n
	if cur == "—" {
		cur = ""
	}
	m.input.SetValue(cur)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m *model) commitEdit() {
	n := m.edit
	val := m.input.Value()
	switch n.kind {
	case kindString:
		n.setStr(strings.TrimSpace(val))
	case kindStringList:
		n.setList(parseCSV(val))
	}
	m.edit = nil
	m.input.Blur()
}

// guardedAction starts an action only if a device is connected.
func (m *model) guardedAction(name string, fn func() error) tea.Cmd {
	if m.status.di == nil {
		m.showMessage("No device", []string{"No Kobo is connected.", "Plug in the device and try again."})
		return nil
	}
	if m.busy {
		return nil
	}
	m.busy = true
	m.appendLog("")
	m.appendLog("$ " + name)
	runAction(m.prog, name, fn)
	return nil
}

func (m *model) onActionDone(msg actionDoneMsg) tea.Cmd {
	m.busy = false
	if msg.err != nil {
		m.appendLog("error: " + msg.err.Error())
	} else {
		m.appendLog("✓ " + msg.name + " complete")
	}
	// After a dry-run, offer to run for real.
	if msg.err == nil {
		switch msg.name {
		case actProvisionDry:
			m.openConfirm("Run provision for real?", func() tea.Cmd {
				return m.guardedAction(actProvision, func() error { return m.actions.Provision(m.mountPath, m.m, false) })
			})
		case actHardenDry:
			m.openConfirm("Apply hardening for real?", func() tea.Cmd {
				return m.guardedAction(actHarden, func() error { return m.actions.Harden(m.mount(), m.m.Hardening, false, false) })
			})
		}
	}
	// Refresh device status after any real action.
	return pollDeviceCmd(m.mountPath, m.m)
}

// --- modals ---

func (m *model) onModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalMessage:
		if k := msg.String(); k == "enter" || k == "esc" {
			m.modal = modalNone
		}
	case modalConfirm:
		switch msg.String() {
		case "y", "enter":
			fn := m.confirmFn
			m.modal = modalNone
			if fn != nil {
				return m, fn()
			}
		case "n", "esc":
			m.modal = modalNone
		}
	case modalDiff:
		switch msg.String() {
		case "enter":
			m.modal = modalNone
			m.saveManifest()
		case "esc":
			m.modal = modalNone
		}
	case modalInstall:
		switch msg.String() {
		case "up", "k":
			if m.modalCursor > 0 {
				m.modalCursor--
			}
		case "down", "j":
			if m.modalCursor < len(m.choices)-1 {
				m.modalCursor++
			}
		case "enter":
			comp := m.choices[m.modalCursor]
			m.modal = modalNone
			return m, m.guardedAction("install "+comp, installComponentFn(comp, m.mount(), m.m))
		case "esc":
			m.modal = modalNone
		}
	case modalPlugins:
		switch msg.String() {
		case "up", "k":
			if m.modalCursor > 0 {
				m.modalCursor--
			}
		case "down", "j":
			if m.modalCursor < len(m.choices)-1 {
				m.modalCursor++
			}
		case " ":
			name := m.choices[m.modalCursor]
			m.pluginChecks[name] = !m.pluginChecks[name]
		case "enter":
			m.applyPlugins()
			m.modal = modalNone
		case "esc":
			m.modal = modalNone
		}
	}
	return m, nil
}

func (m *model) showMessage(title string, lines []string) {
	m.modal = modalMessage
	m.modalTitle = title
	m.modalLines = lines
}

func (m *model) openConfirm(title string, fn func() tea.Cmd) {
	m.modal = modalConfirm
	m.modalTitle = title
	m.confirmFn = fn
}

func (m *model) openInstallMenu() {
	m.modal = modalInstall
	m.modalTitle = "Install component"
	m.choices = []string{"kfmon", "koreader", "nickelmenu"}
	m.modalCursor = 0
}

func (m *model) openPlugins(n *node) {
	m.modal = modalPlugins
	m.modalTitle = "KOReader plugins"
	m.choices = plugins.Names()
	m.modalCursor = 0
	m.pluginChecks = map[string]bool{}
	for _, entry := range n.getList() {
		name, _ := plugins.Parse(entry)
		m.pluginChecks[name] = true
	}
}

// applyPlugins writes the checked plugin names back into the manifest, preserving
// any existing version pins for still-selected plugins.
func (m *model) applyPlugins() {
	pins := map[string]string{}
	for _, entry := range m.m.KOReader.Plugins {
		name, ver := plugins.Parse(entry)
		pins[name] = ver
	}
	var out []string
	for _, name := range m.choices {
		if m.pluginChecks[name] {
			if ver := pins[name]; ver != "" {
				out = append(out, name+"@"+ver)
			} else {
				out = append(out, name)
			}
		}
	}
	m.m.KOReader.Plugins = out
	m.rebuildTree()
}

func (m *model) confirmBackup() tea.Cmd {
	if m.status.di == nil {
		m.showMessage("No device", []string{"Connect a Kobo before backing up."})
		return nil
	}
	di := m.status.di
	m.openConfirm("Back up the device now?", func() tea.Cmd {
		return m.guardedAction("backup", backupFn(di))
	})
	return nil
}

// openSaveDiff validates the manifest, then shows a diff modal (or an error).
func (m *model) openSaveDiff() {
	if errs := manifest.ValidateManifest(m.m); len(errs) > 0 {
		lines := []string{"Fix these before saving:", ""}
		for _, e := range errs {
			lines = append(lines, "• "+e.Error())
		}
		m.showMessage("Invalid config", lines)
		return
	}
	oldR, _ := initcmd.Render(m.original)
	newR, _ := initcmd.Render(*m.m)
	changes := changedOnly(lineDiff(oldR, newR))
	if len(changes) == 0 {
		m.showMessage("No changes", []string{"The manifest matches what's on disk."})
		return
	}
	m.modal = modalDiff
	m.modalTitle = "Save changes to " + m.saveTarget() + "?"
	m.modalLines = nil
	for _, c := range changes {
		m.modalLines = append(m.modalLines, string(c.kind)+" "+c.text)
	}
}

// saveTarget is where a Save will write: the connected device (device-primary)
// or the host fallback path.
func (m *model) saveTarget() string {
	if m.status.di != nil {
		return mstore.DevicePath(m.status.di.MountPoint)
	}
	return m.hostPath
}

func (m *model) saveManifest() {
	path, err := mstore.Save(m.m, m.hostPath, m.status.di)
	if err != nil {
		m.showMessage("Save error", []string{err.Error()})
		return
	}
	m.original = *m.m
	m.manifestPath = path
	m.appendLog("saved " + path)
}

func (m *model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	if m.modal != modalNone {
		return m.viewModal()
	}

	leftW := m.width * 55 / 100
	rightW := m.width - leftW - 4
	panesH := m.height - m.log.Height - 5
	if panesH < 6 {
		panesH = 6
	}

	left := m.viewTree(leftW, panesH)
	right := m.viewStatus(rightW, panesH)

	leftPane := m.paneStyle(focusTree).Width(leftW).Height(panesH).Render(left)
	rightPane := m.st.paneBlurred.Width(rightW).Height(panesH).Render(right) // status pane is display-only
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	logPane := m.paneStyle(focusLog).Width(m.width - 4).Height(m.log.Height).Render(m.log.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewHeader(),
		panes,
		m.viewActionBar(),
		logPane,
	)
}

func (m *model) paneStyle(f focusArea) lipgloss.Style {
	if m.focus == f {
		return m.st.paneFocused
	}
	return m.st.paneBlurred
}

func (m *model) viewHeader() string {
	title := "koboctl"
	if m.dirty() {
		title += " ●"
	}
	hint := "  tab: switch pane · ?: keys"
	return m.st.title.Render(title) + m.st.dim.Render(hint)
}

func (m *model) viewActionBar() string {
	items := "[P]rovision  [I]nstall  [H]arden  [B]ackup  [S]ave  [Q]uit"
	if m.busy {
		items = m.st.cursor.Render("● running…  ") + m.st.dim.Render(items)
		return m.st.actionBar.Render(items)
	}
	return m.st.actionBar.Render(items)
}

// viewTree renders the config tree with a cursor and a simple scroll window.
func (m *model) viewTree(w, h int) string {
	// Keep the cursor within the visible window.
	if m.cursor < m.treeTop {
		m.treeTop = m.cursor
	}
	if m.cursor >= m.treeTop+h {
		m.treeTop = m.cursor - h + 1
	}
	var b strings.Builder
	end := m.treeTop + h
	if end > len(m.flat) {
		end = len(m.flat)
	}
	for i := m.treeTop; i < end; i++ {
		n := m.flat[i]
		gutter := "  "
		if i == m.cursor && m.focus == focusTree {
			gutter = m.st.cursor.Render("❯ ")
		}
		b.WriteString(gutter + m.renderRow(n) + "\n")
	}
	return b.String()
}

// renderRow renders a node body, showing the live text input when it is being
// edited.
func (m *model) renderRow(n *node) string {
	indent := strings.Repeat("  ", n.depth)
	if n.kind == kindGroup {
		caret := "▸"
		if n.expanded {
			caret = "▾"
		}
		return indent + caret + " " + m.st.group.Render(n.label)
	}
	val := n.value()
	if m.edit == n {
		val = m.input.View()
	}
	row := indent + m.st.fieldName.Render(n.label) + "  " + m.st.fieldVal.Render(val)
	if n.note != "" {
		row += m.st.dim.Render("  (" + n.note + ")")
	}
	return row
}

func (m *model) viewStatus(w, h int) string {
	var b strings.Builder
	b.WriteString(m.st.group.Render("Device") + "\n")
	if m.status.err != nil || m.status.di == nil {
		b.WriteString(m.st.bad.Render("○ no device") + "\n")
	} else {
		di := m.status.di
		name := di.Model
		if di.Profile != nil {
			name = di.Profile.Name + " (" + di.Model + ")"
		}
		b.WriteString(m.st.ok.Render("● connected") + "\n")
		b.WriteString(name + "\n")
		b.WriteString(m.st.dim.Render("fw ") + di.FirmwareVersion + "\n")
		b.WriteString(m.st.dim.Render(di.MountPoint) + "\n")
		b.WriteString("\n" + m.st.dim.Render("─ components ─") + "\n")
		for _, c := range m.status.comps {
			mark := m.st.bad.Render("✗")
			if c.installed {
				mark = m.st.ok.Render("✓")
			}
			line := mark + " " + c.name
			if c.version != "" {
				line += m.st.dim.Render(" " + c.version)
			}
			b.WriteString(line + "\n")
		}
		if m.status.hard != nil {
			b.WriteString("\n" + m.st.dim.Render("─ hardening ─") + "\n")
			b.WriteString(hardRow(m.st, "KoboRoot guard", m.status.hard.KoboRootGuarded))
			b.WriteString(hardRow(m.st, "OTA disabled", m.status.hard.OTADisabled))
			b.WriteString(hardRow(m.st, "sync disabled", m.status.hard.SyncDisabled))
			b.WriteString(hardRow(m.st, "boot hook", m.status.hard.BootHookConfigured))
			b.WriteString(hardRow(m.st, "parental", m.status.hard.ParentalEnabled))
		}
	}
	return b.String()
}

func hardRow(st styles, label string, on bool) string {
	mark := st.bad.Render("✗")
	if on {
		mark = st.ok.Render("✓")
	}
	return mark + " " + label + "\n"
}

func (m *model) viewModal() string {
	var b strings.Builder
	b.WriteString(m.st.title.Render(m.modalTitle) + "\n\n")
	switch m.modal {
	case modalMessage:
		for _, l := range m.modalLines {
			b.WriteString(l + "\n")
		}
		b.WriteString("\n" + m.st.dim.Render("[enter] ok"))
	case modalConfirm:
		b.WriteString(m.st.dim.Render("[y] yes   [n] no"))
	case modalDiff:
		for _, l := range m.modalLines {
			switch {
			case strings.HasPrefix(l, "+"):
				b.WriteString(m.st.diffAdd.Render(l) + "\n")
			case strings.HasPrefix(l, "-"):
				b.WriteString(m.st.diffDel.Render(l) + "\n")
			default:
				b.WriteString(l + "\n")
			}
		}
		b.WriteString("\n" + m.st.dim.Render("[enter] save   [esc] cancel"))
	case modalInstall:
		for i, c := range m.choices {
			cur := "  "
			if i == m.modalCursor {
				cur = m.st.cursor.Render("❯ ")
			}
			b.WriteString(cur + c + "\n")
		}
		b.WriteString("\n" + m.st.dim.Render("[enter] install   [esc] cancel"))
	case modalPlugins:
		for i, c := range m.choices {
			cur := "  "
			if i == m.modalCursor {
				cur = m.st.cursor.Render("❯ ")
			}
			check := "[ ]"
			if m.pluginChecks[c] {
				check = m.st.ok.Render("[✓]")
			}
			b.WriteString(cur + check + " " + c + "\n")
		}
		b.WriteString("\n" + m.st.dim.Render("[space] toggle   [enter] apply   [esc] cancel"))
	}
	box := m.st.modal.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
