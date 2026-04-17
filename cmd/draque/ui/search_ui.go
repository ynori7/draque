package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ynori7/draque/domain"
)

// ---- model ----

const searchPageSize = 15

type searchModel struct {
	textInput textinput.Model
	focusCmd  tea.Cmd
	all       []domain.EndpointTemplate
	filtered  []domain.EndpointTemplate
	cursor    int
	scrollOff int
	selected  int // index into all; -1 means no selection
}

func newSearchModel(results []domain.EndpointTemplate) searchModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter by path prefix\u2026"
	ti.SetWidth(60)

	styles := ti.Styles()
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	styles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ti.SetStyles(styles)

	focusCmd := ti.Focus() // sets focused state; returns blink cmd

	return searchModel{
		textInput: ti,
		focusCmd:  focusCmd,
		all:       results,
		filtered:  results,
		cursor:    0,
		scrollOff: 0,
		selected:  -1,
	}
}

func (m searchModel) Init() tea.Cmd {
	return m.focusCmd
}

func (m searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, func() tea.Msg { return searchDoneMsg{endpoint: nil} }

		case "enter":
			if len(m.filtered) > 0 {
				selected := m.filtered[m.cursor]
				for i, ep := range m.all {
					if ep.PathTemplate == selected.PathTemplate && ep.Method == selected.Method {
						m.selected = i
						break
					}
				}
				ep := m.all[m.selected]
				return m, func() tea.Msg { return searchDoneMsg{endpoint: &ep} }
			}
			return m, nil

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scrollOff {
					m.scrollOff = m.cursor
				}
			}
			return m, nil

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.scrollOff+searchPageSize {
					m.scrollOff++
				}
			}
			return m, nil
		}
	}

	// Let the text input handle this keystroke.
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Re-filter on every input change.
	prefix := strings.ToLower(m.textInput.Value())
	m.filtered = nil
	for _, ep := range m.all {
		if prefix == "" || strings.HasPrefix(strings.ToLower(ep.PathTemplate), prefix) {
			m.filtered = append(m.filtered, ep)
		}
	}

	// Keep cursor in bounds and reset scroll on filter change.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.scrollOff = 0

	return m, cmd
}

func (m searchModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m searchModel) render() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(m.textInput.View())
	sb.WriteString("\n\n")

	if len(m.filtered) == 0 {
		sb.WriteString("  " + warnStyle.Render("No endpoints match.") + "\n")
		sb.WriteString("\n  " + subtleStyle.Render("esc to go back") + "\n")
		return sb.String()
	}

	countLine := matchCountStyle.Render(
		fmt.Sprintf("  %d matching endpoint(s)  (↑↓ navigate · enter to select · esc to go back)", len(m.filtered)),
	)
	sb.WriteString(countLine + "\n\n")

	end := m.scrollOff + searchPageSize
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	visible := m.filtered[m.scrollOff:end]

	for relIdx, ep := range visible {
		absIdx := m.scrollOff + relIdx

		methStr := fmt.Sprintf("%-7s", ep.Method)
		methodRendered := methodColor(ep.Method).Render(methStr)
		countRendered := subtleStyle.Render(fmt.Sprintf("count: %d", ep.Count))

		var line string
		if absIdx == m.cursor {
			line = fmt.Sprintf("  %s  %s  %-52s  %s",
				cursorStyle.Render("▶"),
				methodRendered,
				selectedStyle.Render(ep.PathTemplate),
				countRendered,
			)
		} else {
			line = fmt.Sprintf("     %s  %-52s  %s",
				methodRendered,
				pathStyle.Render(ep.PathTemplate),
				countRendered,
			)
		}
		sb.WriteString(line + "\n")
	}

	// Scroll indicators
	if m.scrollOff > 0 {
		sb.WriteString("  " + subtleStyle.Render("  ↑ more above") + "\n")
	}
	if end < len(m.filtered) {
		sb.WriteString("  " + subtleStyle.Render("  ↓ more below") + "\n")
	}
	sb.WriteString("\n  " + subtleStyle.Render("esc to go back") + "\n")

	return sb.String()
}

// max returns the larger of two ints (Go 1.21+ has a builtin, but this keeps compat).
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
