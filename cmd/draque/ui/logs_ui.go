package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/ynori7/draque/domain"
)

type logsStep int

const (
	logsStepFilePath      logsStep = iota
	logsStepPatternChoice          // cursor-based menu (only if prev patterns exist)
	logsStepPatternInput           // text input for new/edited pattern
)

type logsModel struct {
	step         logsStep
	fileInput    textinput.Model
	patternInput textinput.Model
	initCmd      tea.Cmd
	filePath     string   // stored after validation
	prevPatterns []string // patterns from existing log sources
	menuCursor   int      // cursor on the pattern choice menu
	fileErr      string
	patternErr   string
}

func newLogsModel(prevPatterns []string) logsModel {
	fi := textinput.New()
	fi.Placeholder = "/var/log/access.log"
	fi.Prompt = "  Log file path: "
	fi.SetWidth(60)
	fs := fi.Styles()
	fs.Focused.Prompt = subtleStyle
	fs.Focused.Text = inputTextStyle
	fi.SetStyles(fs)

	pi := textinput.New()
	pi.Placeholder = `{host} {method} {path} {status}`
	pi.Prompt = "  Pattern: "
	pi.SetWidth(60)
	ps := pi.Styles()
	ps.Focused.Prompt = subtleStyle
	ps.Focused.Text = inputTextStyle
	pi.SetStyles(ps)

	initCmd := fi.Focus()
	return logsModel{
		step:         logsStepFilePath,
		fileInput:    fi,
		patternInput: pi,
		initCmd:      initCmd,
		prevPatterns: prevPatterns,
	}
}

func (m logsModel) Init() tea.Cmd {
	return m.initCmd
}

func (m logsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return subCancelMsg{} }

		case "enter":
			return m.handleEnter()

		case "up", "ctrl+p":
			if m.step == logsStepPatternChoice && m.menuCursor > 0 {
				m.menuCursor--
			}
			return m, nil

		case "down", "ctrl+n":
			if m.step == logsStepPatternChoice {
				max := len(m.prevPatterns) // cursor 0 = new, 1..N = reuse
				if m.menuCursor < max {
					m.menuCursor++
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case logsStepFilePath:
		m.fileInput, cmd = m.fileInput.Update(msg)
	case logsStepPatternInput:
		m.patternInput, cmd = m.patternInput.Update(msg)
	}
	return m, cmd
}

func (m logsModel) handleEnter() (logsModel, tea.Cmd) {
	switch m.step {
	case logsStepFilePath:
		p := strings.TrimSpace(m.fileInput.Value())
		if p == "" {
			m.fileErr = "File path is required."
			return m, nil
		}
		if _, err := os.Stat(p); err != nil {
			m.fileErr = fmt.Sprintf("File not found: %s", p)
			return m, nil
		}
		m.fileErr = ""
		m.filePath = p
		if len(m.prevPatterns) > 0 {
			m.step = logsStepPatternChoice
			m.menuCursor = 0
			return m, nil
		}
		m.step = logsStepPatternInput
		return m, m.patternInput.Focus()

	case logsStepPatternChoice:
		if m.menuCursor == 0 {
			// User wants to enter a new pattern.
			m.step = logsStepPatternInput
			return m, m.patternInput.Focus()
		}
		// Reuse an existing pattern.
		pattern := m.prevPatterns[m.menuCursor-1]
		return m.validateAndFinish(pattern)

	case logsStepPatternInput:
		pattern := strings.TrimSpace(m.patternInput.Value())
		if pattern == "" {
			m.patternErr = "Pattern is required."
			return m, nil
		}
		return m.validateAndFinish(pattern)
	}
	return m, nil
}

// validateAndFinish compiles + first-line-validates pattern then emits done or error.
func (m logsModel) validateAndFinish(pattern string) (logsModel, tea.Cmd) {
	compiled, err := domain.CompileAccessLogPattern(pattern)
	if err != nil {
		m.patternErr = fmt.Sprintf("Invalid pattern: %v", err)
		if m.step == logsStepPatternChoice {
			// Show error on choice step; user can try another option.
		}
		return m, nil
	}
	if err := validateFirstLogLine(m.filePath, compiled); err != nil {
		m.patternErr = err.Error() + " — please choose or enter a different pattern."
		if m.step == logsStepPatternChoice {
			// Stay on choice menu so the user can try another option.
		} else {
			// Stay on input step.
		}
		return m, nil
	}
	m.patternErr = ""
	src := logSource{FilePath: m.filePath, Pattern: pattern}
	return m, func() tea.Msg { return logsDoneMsg{source: src} }
}

func (m logsModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m logsModel) render() string {
	var sb strings.Builder
	sb.WriteString("\n  " + headerStyle.Render("Add access log source") + "\n\n")

	sb.WriteString(m.fileInput.View() + "\n")
	if m.fileErr != "" {
		sb.WriteString("  " + errorStyle.Render(m.fileErr) + "\n")
	}

	if m.step == logsStepPatternChoice {
		sb.WriteString("\n  " + subtleStyle.Render("Log format pattern — reuse or enter new:") + "\n\n")
		items := append([]string{"[0]  Enter a new format"}, func() []string {
			out := make([]string, len(m.prevPatterns))
			for i, p := range m.prevPatterns {
				out[i] = fmt.Sprintf("[%d]  %s", i+1, p)
			}
			return out
		}()...)
		for i, item := range items {
			prefix := "     "
			if i == m.menuCursor {
				prefix = "  " + cursorStyle.Render("▶") + "  "
				sb.WriteString(prefix + selectedStyle.Render(item) + "\n")
			} else {
				sb.WriteString(prefix + subtleStyle.Render(item) + "\n")
			}
		}
		sb.WriteString("\n  " + subtleStyle.Render("↑↓ to navigate · enter to select") + "\n")
		if m.patternErr != "" {
			sb.WriteString("\n  " + errorStyle.Render(m.patternErr) + "\n")
		}
	}

	if m.step == logsStepPatternInput {
		sb.WriteString("\n  " + subtleStyle.Render("Log format pattern (use {host}, {path}, {method}, {status}, ...):") + "\n")
		sb.WriteString(m.patternInput.View() + "\n")
		if m.patternErr != "" {
			sb.WriteString("  " + errorStyle.Render(m.patternErr) + "\n")
		}
	}

	return sb.String()
}
