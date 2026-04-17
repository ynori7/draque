package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type swaggerModel struct {
	input   textinput.Model
	initCmd tea.Cmd
	errMsg  string
}

func newSwaggerModel() swaggerModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/swagger.json"
	ti.Prompt = "  File path: "
	ti.SetWidth(60)
	ts := ti.Styles()
	ts.Focused.Prompt = subtleStyle
	ts.Focused.Text = inputTextStyle
	ti.SetStyles(ts)

	initCmd := ti.Focus()
	return swaggerModel{input: ti, initCmd: initCmd}
}

func (m swaggerModel) Init() tea.Cmd {
	return m.initCmd
}

func (m swaggerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return subCancelMsg{} }

		case "enter":
			p := strings.TrimSpace(m.input.Value())
			if p == "" {
				m.errMsg = "File path is required."
				return m, nil
			}
			if _, err := os.Stat(p); err != nil {
				m.errMsg = fmt.Sprintf("File not found: %s", p)
				return m, nil
			}
			return m, func() tea.Msg { return swaggerDoneMsg{filePath: p} }
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m swaggerModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m swaggerModel) render() string {
	var sb strings.Builder
	sb.WriteString("\n  " + headerStyle.Render("Add Swagger/OpenAPI source") + "\n\n")
	sb.WriteString(m.input.View() + "\n")
	if m.errMsg != "" {
		sb.WriteString("  " + errorStyle.Render(m.errMsg) + "\n")
	}
	return sb.String()
}
