package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ynori7/draque/domain"
)

// ---- mode enum ----

type appMode int

const (
	modeMain    appMode = iota
	modeWayback         // wayback source input
	modeLogs            // access log source input
	modeSwagger         // swagger source input
	modeScan            // scan progress
	modeSearch          // interactive search
	modeExport          // export flow
)

// ---- inter-model completion messages ----
// Sub-models signal results by returning a Cmd that yields one of these.

type waybackDoneMsg struct{ source waybackSource }
type logsDoneMsg struct{ source logSource }
type logsDirDoneMsg struct {
	dir     string
	sources []logSource
}
type swaggerDoneMsg struct{ filePath string }
type swaggerDirDoneMsg struct {
	dir   string
	files []string
}
type scanFinishedMsg struct{ results []domain.EndpointTemplate }
type searchDoneMsg struct{ endpoint *domain.EndpointTemplate } // nil = user went back
type exportDoneMsg struct {
	count int
	path  string
}
type subCancelMsg struct{} // any sub-model cancelled via Esc

// ---- root model ----

type appModel struct {
	state     *appState
	mode      appMode
	input     textinput.Model
	initCmd   tea.Cmd // stored focus+blink cmd from newAppModel
	output    string  // text shown in the content area when in modeMain
	width     int
	height    int
	activeSub tea.Model // current sub-model, nil in modeMain
}

// headerHeight is the number of lines rendered by buildHeader().
//
// blank + banner + blank + pre-scan cmds + post-scan cmds + blank + separator + blank = 8
const headerHeight = 8

// footerHeight is the number of lines rendered by buildFooter().
// blank + prompt = 2
const footerHeight = 2

func NewAppModel() appModel {
	ti := textinput.New()
	ti.Prompt = "draque> "
	ti.SetWidth(60)
	ts := ti.Styles()
	ts.Focused.Prompt = promptStyle
	ts.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ti.SetStyles(ts)

	// Pre-focus so the stored model has focus=true from the start.
	// Init() will return this cmd (cursor blink) as the initial command.
	blinkCmd := ti.Focus()

	return appModel{
		state: &appState{
			limits: domain.ScanLimits{
				MaxObservations: 10,
				MaxExamples:     10,
			},
		},
		mode:    modeMain,
		input:   ti,
		initCmd: blinkCmd,
	}
}

// ---- Init ----

func (m appModel) Init() tea.Cmd {
	return m.initCmd
}

// ---- Update ----

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.mode == modeMain {
		return m.updateMain(msg)
	}
	return m.updateSubMode(msg)
}

func (m appModel) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		cmd := strings.TrimSpace(m.input.Value())
		m.input.SetValue("")
		return m.dispatch(cmd)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m appModel) dispatch(cmd string) (appModel, tea.Cmd) {
	switch cmd {
	case "":
		return m, nil

	case "help", "h", "?":
		m.output = renderHelp(m.state.scanned)
		return m, nil

	case "wayback", "w":
		sub := newWaybackModel()
		m.activeSub = sub
		m.mode = modeWayback
		return m, sub.Init()

	case "logs", "log", "l":
		sub := newLogsModel(existingPatterns(m.state))
		m.activeSub = sub
		m.mode = modeLogs
		return m, sub.Init()

	case "swagger", "sw":
		sub := newSwaggerModel()
		m.activeSub = sub
		m.mode = modeSwagger
		return m, sub.Init()

	case "status":
		m.output = renderStatus(m.state)
		return m, nil

	case "scan", "s":
		if len(m.state.waybackSources) == 0 && len(m.state.logSources) == 0 && len(m.state.swaggerSources) == 0 {
			m.output = "  " + warnStyle.Render("No sources configured. Use 'wayback', 'logs', or 'swagger' to add sources first.")
			return m, nil
		}
		sub := newScanModel(m.state)
		m.activeSub = sub
		m.mode = modeScan
		return m, sub.Init()

	case "analyze", "a":
		if !m.state.scanned {
			m.output = "  " + warnStyle.Render("No scan results yet. Run 'scan' first.")
			return m, nil
		}
		m.output = renderAnalyze(m.state)
		return m, nil

	case "search":
		if !m.state.scanned {
			m.output = "  " + warnStyle.Render("No scan results yet. Run 'scan' first.")
			return m, nil
		}
		sub := newSearchModel(m.state.results)
		m.activeSub = sub
		m.mode = modeSearch
		return m, sub.Init()

	case "export", "e":
		if !m.state.scanned {
			m.output = "  " + warnStyle.Render("No scan results yet. Run 'scan' first.")
			return m, nil
		}
		sub := newExportModel(m.state.results)
		m.activeSub = sub
		m.mode = modeExport
		return m, sub.Init()

	case "reset", "r":
		if !m.state.scanned && len(m.state.waybackSources) == 0 && len(m.state.logSources) == 0 && len(m.state.swaggerSources) == 0 {
			m.output = "  " + subtleStyle.Render("Nothing to reset.")
			return m, nil
		}
		m.state.Reset()
		m.output = "  " + successStyle.Render("All sources and scan data cleared.")
		return m, nil

	case "quit", "exit", "q":
		return m, tea.Quit

	default:
		m.output = "  " + warnStyle.Render(fmt.Sprintf("Unknown command %q. Type 'help' for available commands.", cmd))
		return m, nil
	}
}

func (m appModel) updateSubMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle completion/cancel messages before delegating to sub-model.
	switch msg := msg.(type) {
	case subCancelMsg:
		m.mode = modeMain
		m.activeSub = nil
		return m, m.input.Focus()

	case waybackDoneMsg:
		m.state.waybackSources = append(m.state.waybackSources, msg.source)
		m.mode = modeMain
		m.activeSub = nil
		m.output = fmt.Sprintf("  %s %s%s",
			successStyle.Render("Added wayback source:"),
			waybackStyle.Render(msg.source.Domain),
			subtleStyle.Render(msg.source.PathPrefix),
		)
		return m, m.input.Focus()

	case logsDoneMsg:
		m.state.logSources = append(m.state.logSources, msg.source)
		m.mode = modeMain
		m.activeSub = nil
		m.output = fmt.Sprintf("  %s %s",
			successStyle.Render("Added log source:"),
			logsStyle.Render(msg.source.FilePath),
		)
		return m, m.input.Focus()

	case logsDirDoneMsg:
		m.state.logSources = append(m.state.logSources, msg.sources...)
		m.mode = modeMain
		m.activeSub = nil
		m.output = fmt.Sprintf("  %s %s",
			successStyle.Render(fmt.Sprintf("Added %d log source(s) from directory:", len(msg.sources))),
			logsStyle.Render(msg.dir),
		)
		return m, m.input.Focus()

	case swaggerDoneMsg:
		m.state.swaggerSources = append(m.state.swaggerSources, msg.filePath)
		m.mode = modeMain
		m.activeSub = nil
		m.output = fmt.Sprintf("  %s %s",
			successStyle.Render("Added swagger source:"),
			swaggerStyle.Render(msg.filePath),
		)
		return m, m.input.Focus()

	case swaggerDirDoneMsg:
		m.state.swaggerSources = append(m.state.swaggerSources, msg.files...)
		m.mode = modeMain
		m.activeSub = nil
		m.output = fmt.Sprintf("  %s %s",
			successStyle.Render(fmt.Sprintf("Added %d swagger sources from directory:", len(msg.files))),
			swaggerStyle.Render(msg.dir),
		)
		return m, m.input.Focus()

	case scanFinishedMsg:
		if len(msg.results) > 0 {
			m.state.results = msg.results
			m.state.scanned = true
		}
		m.mode = modeMain
		m.activeSub = nil
		if m.state.scanned {
			m.output = fmt.Sprintf("\n  %s",
				successStyle.Bold(true).Render(fmt.Sprintf("Scan complete — %d unique endpoint templates.", len(m.state.results))),
			)
		} else {
			m.output = "  " + warnStyle.Render("All sources failed. No results.")
		}
		return m, m.input.Focus()

	case searchDoneMsg:
		m.mode = modeMain
		m.activeSub = nil
		if msg.endpoint != nil {
			m.output = renderEndpointDetail(*msg.endpoint)
		}
		return m, m.input.Focus()

	case exportDoneMsg:
		m.mode = modeMain
		m.activeSub = nil
		m.output = fmt.Sprintf("  %s %s",
			successStyle.Render(fmt.Sprintf("Exported %d URLs to", msg.count)),
			headerStyle.Render(msg.path),
		)
		return m, m.input.Focus()
	}

	// Delegate to the active sub-model.
	if m.activeSub != nil {
		newSub, cmd := m.activeSub.Update(msg)
		m.activeSub = newSub
		return m, cmd
	}
	return m, nil
}

// existingPatterns returns the unique patterns from all configured log sources.
func existingPatterns(state *appState) []string {
	var patterns []string
	seen := map[string]bool{}
	for _, s := range state.logSources {
		if !seen[s.Pattern] {
			patterns = append(patterns, s.Pattern)
			seen[s.Pattern] = true
		}
	}
	return patterns
}

// ---- View ----

func (m appModel) View() tea.View {
	if m.width == 0 {
		view := tea.NewView("")
		view.AltScreen = true
		return view
	}
	var sb strings.Builder

	sb.WriteString(m.buildHeader())

	// Content area: fills the space between header and footer.
	contentLines := m.height - headerHeight - footerHeight
	if contentLines < 0 {
		contentLines = 0
	}

	var raw string
	if m.mode == modeMain {
		raw = m.output
	} else if m.activeSub != nil {
		raw = m.activeSub.View().Content
	}

	lines := strings.Split(raw, "\n")
	// Strip trailing blank lines from the source content.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// Truncate if too tall.
	if len(lines) > contentLines {
		lines = lines[:contentLines]
	}
	// Pad to exactly contentLines so the footer is always at the bottom.
	for len(lines) < contentLines {
		lines = append(lines, "")
	}
	sb.WriteString(strings.Join(lines, "\n"))
	sb.WriteString("\n")

	sb.WriteString(m.buildFooter())
	view := tea.NewView(sb.String())
	view.AltScreen = true
	return view
}

// buildHeader returns the fixed 8-line header block (ends with a newline).
func (m appModel) buildHeader() string {
	sep := subtleStyle.Render(strings.Repeat("─", max2(m.width-4, 1)))

	// Pre-scan commands (always shown)
	pre := []string{"wayback", "logs", "swagger", "status", "scan", "help", "quit"}
	preLine := joinCmds(pre)

	// Post-scan commands (only after scan)
	var postLine string
	if m.state.scanned {
		post := []string{"analyze", "search", "export", "reset"}
		postLine = "             " + joinCmds(post)
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(bannerStyle.Render("  Draque — API endpoint discovery tool") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + headerStyle.Render("Commands:") + "  " + preLine + "\n")
	sb.WriteString(postLine + "\n") // blank if not scanned
	sb.WriteString("\n")
	sb.WriteString("  " + sep + "\n")
	sb.WriteString("\n")
	return sb.String()
}

func joinCmds(names []string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = cmdNameStyle.Render(n)
	}
	return strings.Join(parts, subtleStyle.Render("  ·  "))
}

// buildFooter returns the fixed 2-line footer block (ends with a newline).
func (m appModel) buildFooter() string {
	var sb strings.Builder
	sb.WriteString("\n")
	if m.mode == modeMain {
		sb.WriteString("  " + m.input.View() + "\n")
	} else {
		sb.WriteString("  " + subtleStyle.Render("(esc to cancel · ctrl+c to quit)") + "\n")
	}
	return sb.String()
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
