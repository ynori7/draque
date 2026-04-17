package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/ynori7/draque/domain"
)

type waybackStep int

const (
	waybackStepDomain waybackStep = iota
	waybackStepPrefix
)

type waybackModel struct {
	step        waybackStep
	domainInput textinput.Model
	prefixInput textinput.Model
	initCmd     tea.Cmd
	errMsg      string
}

func newWaybackModel() waybackModel {
	di := textinput.New()
	di.Placeholder = "e.g. example.com"
	di.Prompt = "  Domain (e.g. example.com): "
	di.SetWidth(40)
	dStyles := di.Styles()
	dStyles.Focused.Prompt = subtleStyle
	dStyles.Focused.Text = inputTextStyle
	di.SetStyles(dStyles)

	pi := textinput.New()
	pi.Placeholder = "optional — press enter to skip"
	pi.Prompt = "  Path prefix (optional): "
	pi.SetWidth(40)
	pStyles := pi.Styles()
	pStyles.Focused.Prompt = subtleStyle
	pStyles.Focused.Text = inputTextStyle
	pi.SetStyles(pStyles)

	initCmd := di.Focus()
	return waybackModel{
		step:        waybackStepDomain,
		domainInput: di,
		prefixInput: pi,
		initCmd:     initCmd,
	}
}

func (m waybackModel) Init() tea.Cmd {
	return m.initCmd
}

func (m waybackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return subCancelMsg{} }

		case "enter":
			switch m.step {
			case waybackStepDomain:
				d := strings.TrimSpace(m.domainInput.Value())
				if d == "" {
					m.errMsg = "Domain is required."
					return m, nil
				}
				if err := domain.ValidateWaybackDomain(d); err != nil {
					m.errMsg = fmt.Sprintf("Invalid domain: %v", err)
					return m, nil
				}
				m.errMsg = ""
				m.step = waybackStepPrefix
				return m, m.prefixInput.Focus()

			case waybackStepPrefix:
				src := waybackSource{
					Domain:     strings.TrimSpace(m.domainInput.Value()),
					PathPrefix: strings.TrimSpace(m.prefixInput.Value()),
				}
				return m, func() tea.Msg { return waybackDoneMsg{source: src} }
			}
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case waybackStepDomain:
		m.domainInput, cmd = m.domainInput.Update(msg)
	case waybackStepPrefix:
		m.prefixInput, cmd = m.prefixInput.Update(msg)
	}
	return m, cmd
}

func (m waybackModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m waybackModel) render() string {
	var sb strings.Builder
	sb.WriteString("\n  " + headerStyle.Render("Add Wayback Machine source") + "\n\n")
	sb.WriteString(m.domainInput.View() + "\n")
	if m.errMsg != "" {
		sb.WriteString("  " + errorStyle.Render(m.errMsg) + "\n")
	}
	sb.WriteString("\n")
	if m.step == waybackStepPrefix {
		sb.WriteString(m.prefixInput.View() + "\n")
		sb.WriteString("\n  " + subtleStyle.Render("press enter to skip") + "\n")
	}
	return sb.String()
}
