package ui

import (
	"fmt"
	"os"
	"path/filepath"
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
	ti.Placeholder = "/path/to/swagger.json  or  /path/to/specs/"
	ti.Prompt = "  Path: "
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
				m.errMsg = "A file or directory path is required."
				return m, nil
			}
			info, err := os.Stat(p)
			if err != nil {
				m.errMsg = fmt.Sprintf("Path not found: %s", p)
				return m, nil
			}
			if info.IsDir() {
				files, err := swaggerFilesInDir(p)
				if err != nil {
					m.errMsg = fmt.Sprintf("Cannot read directory: %v", err)
					return m, nil
				}
				if len(files) == 0 {
					m.errMsg = "No .json / .yaml / .yml files found in that directory."
					return m, nil
				}
				dir := p
				return m, func() tea.Msg { return swaggerDirDoneMsg{dir: dir, files: files} }
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
	sb.WriteString("  " + subtleStyle.Render("Enter a file path (.json/.yaml/.yml) or a directory — all spec files in the directory will be added.") + "\n\n")
	sb.WriteString(m.input.View() + "\n")
	if m.errMsg != "" {
		sb.WriteString("  " + errorStyle.Render(m.errMsg) + "\n")
	}
	return sb.String()
}

// swaggerFilesInDir returns all .json / .yaml / .yml files directly inside dir.
func swaggerFilesInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch filepath.Ext(e.Name()) {
		case ".json", ".yaml", ".yml":
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}
