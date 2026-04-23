package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ynori7/draque/domain"
)

// ---- model ----

const searchPageSize = 15

type searchMode int

const (
	searchModeRoute searchMode = iota
	searchModeParam
)

// paramEntry is a pre-computed summary of a route parameter for the search index.
type paramEntry struct {
	name       string
	routeCount int
}

type searchModel struct {
	textInput textinput.Model
	focusCmd  tea.Cmd
	mode      searchMode
	// Route mode
	all      []domain.EndpointTemplate
	filtered []domain.EndpointTemplate
	// Param mode
	allParams      []paramEntry
	filteredParams []paramEntry
	cursor         int
	scrollOff      int
	selected       int // index into all; -1 means no selection
}

func newSearchModel(results []domain.EndpointTemplate) searchModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter by path prefix\u2026"
	ti.SetWidth(60)

	styles := ti.Styles()
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	styles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ti.SetStyles(styles)

	focusCmd := ti.Focus()

	return searchModel{
		textInput:      ti,
		focusCmd:       focusCmd,
		mode:           searchModeRoute,
		all:            results,
		filtered:       results,
		allParams:      buildParamEntries(results),
		filteredParams: buildParamEntries(results),
		cursor:         0,
		scrollOff:      0,
		selected:       -1,
	}
}

// buildParamEntries computes a sorted list of paramEntry values from scan results.
// This runs once when entering search mode, not per keystroke.
func buildParamEntries(results []domain.EndpointTemplate) []paramEntry {
	paramRoutes := make(map[string]map[string]struct{})
	for _, ep := range results {
		for _, p := range ep.Parameters {
			if paramRoutes[p.Name] == nil {
				paramRoutes[p.Name] = make(map[string]struct{})
			}
			paramRoutes[p.Name][ep.Method+"\x00"+ep.PathTemplate] = struct{}{}
		}
	}
	entries := make([]paramEntry, 0, len(paramRoutes))
	for name, routes := range paramRoutes {
		entries = append(entries, paramEntry{name: name, routeCount: len(routes)})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].routeCount != entries[j].routeCount {
			return entries[i].routeCount > entries[j].routeCount
		}
		return entries[i].name < entries[j].name
	})
	return entries
}

func (m searchModel) Init() tea.Cmd {
	return m.focusCmd
}

func (m searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, func() tea.Msg { return searchDoneMsg{} }

		case "tab":
			if m.mode == searchModeRoute {
				m.mode = searchModeParam
				m.textInput.Placeholder = "type to filter by parameter name\u2026"
			} else {
				m.mode = searchModeRoute
				m.textInput.Placeholder = "type to filter by path prefix\u2026"
			}
			m.textInput.SetValue("")
			m.cursor = 0
			m.scrollOff = 0
			m.filtered = m.all
			m.filteredParams = m.allParams
			return m, nil

		case "enter":
			if m.mode == searchModeParam {
				if len(m.filteredParams) > 0 {
					selected := m.filteredParams[m.cursor]
					return m, func() tea.Msg {
						return searchDoneMsg{paramName: selected.name}
					}
				}
				return m, nil
			}
			// Route mode
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
			listLen := len(m.filtered)
			if m.mode == searchModeParam {
				listLen = len(m.filteredParams)
			}
			if m.cursor < listLen-1 {
				m.cursor++
				if m.cursor >= m.scrollOff+searchPageSize {
					m.scrollOff++
				}
			}
			return m, nil
		}
	}

	// Let the text input handle this keystroke.
	prevValue := m.textInput.Value()
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Re-filter only when the input value actually changed.
	prefix := strings.ToLower(m.textInput.Value())
	if m.textInput.Value() != prevValue {
		if m.mode == searchModeParam {
			m.filteredParams = nil
			for _, p := range m.allParams {
				if prefix == "" || strings.HasPrefix(strings.ToLower(p.name), prefix) {
					m.filteredParams = append(m.filteredParams, p)
				}
			}
			if m.cursor >= len(m.filteredParams) {
				m.cursor = max(0, len(m.filteredParams)-1)
			}
		} else {
			m.filtered = nil
			for _, ep := range m.all {
				if prefix == "" || strings.HasPrefix(strings.ToLower(ep.PathTemplate), prefix) {
					m.filtered = append(m.filtered, ep)
				}
			}
			if m.cursor >= len(m.filtered) {
				m.cursor = max(0, len(m.filtered)-1)
			}
		}
		m.scrollOff = 0
	}

	return m, cmd
}

func (m searchModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m searchModel) render() string {
	var sb strings.Builder

	// Mode indicator + toggle hint
	modeLabel := "Routes"
	if m.mode == searchModeParam {
		modeLabel = "Parameters"
	}
	sb.WriteString(fmt.Sprintf("\n  %s %s  %s\n",
		headerStyle.Render("Mode:"),
		selectedStyle.Render(modeLabel),
		subtleStyle.Render("(tab to toggle)"),
	))

	sb.WriteString("  ")
	sb.WriteString(m.textInput.View())
	sb.WriteString("\n\n")

	if m.mode == searchModeParam {
		return m.renderParamList(&sb)
	}
	return m.renderRouteList(&sb)
}

func (m searchModel) renderRouteList(sb *strings.Builder) string {
	if len(m.filtered) == 0 {
		sb.WriteString("  " + warnStyle.Render("No endpoints match.") + "\n")
		sb.WriteString("\n  " + subtleStyle.Render("esc to go back") + "\n")
		return sb.String()
	}

	sb.WriteString(matchCountStyle.Render(
		fmt.Sprintf("  %d matching endpoint(s)  (↑↓ navigate · enter to select · esc to go back)", len(m.filtered)),
	) + "\n\n")

	end := m.scrollOff + searchPageSize
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	for relIdx, ep := range m.filtered[m.scrollOff:end] {
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

	if m.scrollOff > 0 {
		sb.WriteString("  " + subtleStyle.Render("  ↑ more above") + "\n")
	}
	if end < len(m.filtered) {
		sb.WriteString("  " + subtleStyle.Render("  ↓ more below") + "\n")
	}
	sb.WriteString("\n  " + subtleStyle.Render("esc to go back") + "\n")
	return sb.String()
}

func (m searchModel) renderParamList(sb *strings.Builder) string {
	if len(m.filteredParams) == 0 {
		sb.WriteString("  " + warnStyle.Render("No parameters match.") + "\n")
		sb.WriteString("\n  " + subtleStyle.Render("esc to go back") + "\n")
		return sb.String()
	}

	sb.WriteString(matchCountStyle.Render(
		fmt.Sprintf("  %d matching parameter(s)  (↑↓ navigate · enter to select · esc to go back)", len(m.filteredParams)),
	) + "\n\n")

	end := m.scrollOff + searchPageSize
	if end > len(m.filteredParams) {
		end = len(m.filteredParams)
	}
	for relIdx, pe := range m.filteredParams[m.scrollOff:end] {
		absIdx := m.scrollOff + relIdx
		routesStr := subtleStyle.Render(fmt.Sprintf("seen in %d route(s)", pe.routeCount))
		var line string
		if absIdx == m.cursor {
			line = fmt.Sprintf("  %s  %-40s  %s",
				cursorStyle.Render("▶"),
				selectedStyle.Render(pe.name),
				routesStr,
			)
		} else {
			line = fmt.Sprintf("     %-40s  %s",
				paramStyle.Render(pe.name),
				routesStr,
			)
		}
		sb.WriteString(line + "\n")
	}

	if m.scrollOff > 0 {
		sb.WriteString("  " + subtleStyle.Render("  ↑ more above") + "\n")
	}
	if end < len(m.filteredParams) {
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
