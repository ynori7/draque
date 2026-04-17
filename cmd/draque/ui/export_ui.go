package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/ynori7/draque/domain"
)

type exportStep int

const (
	exportStepPath   exportStep = iota
	exportStepFilter            // cursor-based choice: all vs 2xx only
)

type exportModel struct {
	step         exportStep
	pathInput    textinput.Model
	initCmd      tea.Cmd
	filterCursor int // 0 = all results, 1 = 2xx only
	results      []domain.EndpointTemplate
	errMsg       string
}

func newExportModel(results []domain.EndpointTemplate) exportModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/output.txt"
	ti.Prompt = "  Output file path: "
	ti.SetWidth(60)
	ts := ti.Styles()
	ts.Focused.Prompt = subtleStyle
	ts.Focused.Text = inputTextStyle
	ti.SetStyles(ts)

	initCmd := ti.Focus()
	return exportModel{
		step:      exportStepPath,
		pathInput: ti,
		initCmd:   initCmd,
		results:   results,
	}
}

func (m exportModel) Init() tea.Cmd {
	return m.initCmd
}

func (m exportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return subCancelMsg{} }

		case "enter":
			return m.handleEnter()

		case "up", "ctrl+p":
			if m.step == exportStepFilter && m.filterCursor > 0 {
				m.filterCursor--
			}
			return m, nil

		case "down", "ctrl+n":
			if m.step == exportStepFilter && m.filterCursor < 1 {
				m.filterCursor++
			}
			return m, nil
		}
	}

	if m.step == exportStepPath {
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m exportModel) handleEnter() (exportModel, tea.Cmd) {
	switch m.step {
	case exportStepPath:
		p := strings.TrimSpace(m.pathInput.Value())
		if p == "" {
			m.errMsg = "File path is required."
			return m, nil
		}
		m.errMsg = ""
		m.step = exportStepFilter
		return m, nil

	case exportStepFilter:
		outputPath := strings.TrimSpace(m.pathInput.Value())
		onlyTwoXX := m.filterCursor == 1

		f, err := os.Create(outputPath)
		if err != nil {
			m.errMsg = fmt.Sprintf("Error creating file: %v", err)
			return m, nil
		}
		defer f.Close()

		written := 0
		for _, ep := range m.results {
			url := pickExampleURL(ep.Observations, onlyTwoXX)
			if url == "" {
				continue
			}
			method := ep.Method
			if method == "" {
				method = "GET"
			}
			fmt.Fprintf(f, "%s %s\n", method, url)
			written++
		}

		count := written
		path := outputPath
		return m, func() tea.Msg { return exportDoneMsg{count: count, path: path} }
	}
	return m, nil
}

func (m exportModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m exportModel) render() string {
	var sb strings.Builder
	sb.WriteString("\n  " + headerStyle.Render("Export endpoint URLs to file") + "\n\n")
	sb.WriteString("  " + subtleStyle.Render("Writes one representative URL per endpoint template (one per line).") + "\n\n")

	sb.WriteString(m.pathInput.View() + "\n")
	if m.errMsg != "" && m.step == exportStepPath {
		sb.WriteString("  " + errorStyle.Render(m.errMsg) + "\n")
	}

	if m.step == exportStepFilter {
		sb.WriteString("\n  " + subtleStyle.Render("Filter:") + "\n\n")
		filters := []string{
			"All results",
			"2xx responses only  (falls back to unknown status if no 2xx available)",
		}
		for i, label := range filters {
			if i == m.filterCursor {
				sb.WriteString("  " + cursorStyle.Render("▶") + "  " + selectedStyle.Render(label) + "\n")
			} else {
				sb.WriteString("     " + subtleStyle.Render(label) + "\n")
			}
		}
		sb.WriteString("\n  " + subtleStyle.Render("↑↓ to select · enter to confirm") + "\n")
		if m.errMsg != "" {
			sb.WriteString("\n  " + errorStyle.Render(m.errMsg) + "\n")
		}
	}

	return sb.String()
}
