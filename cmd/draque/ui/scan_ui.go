package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ynori7/draque/domain"
)

// ---- messages sent from the scan goroutine ----

type sourceStartMsg struct{ label string }
type sourceDoneMsg struct {
	label string
	count int
}
type scanErrMsg struct {
	label string
	err   error
}
type scanDoneMsg struct{ results [][]domain.EndpointTemplate }

// ---- model ----

type completedEntry struct {
	label string
	count int
	isErr bool
}

type scanModel struct {
	state      *appState
	ch         chan tea.Msg // goroutine → bubbletea
	spinner    spinner.Model
	progress   progress.Model
	totalSteps int
	stepsDone  int
	current    string
	completed  []completedEntry
	allResults [][]domain.EndpointTemplate
	done       bool
	quitting   bool
}

func newScanModel(state *appState) scanModel {
	total := len(state.waybackSources) + len(state.logSources) + len(state.swaggerSources)
	ch := make(chan tea.Msg, 64)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	pg := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(46),
		progress.WithoutPercentage(),
	)

	return scanModel{
		state:      state,
		ch:         ch,
		spinner:    sp,
		progress:   pg,
		totalSteps: total,
	}
}

// waitForActivity returns a Cmd that blocks until the scan goroutine sends
// the next message, then delivers it to the Update loop.
func waitForActivity(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil // channel closed — no-op
		}
		return msg
	}
}

func (m scanModel) Init() tea.Cmd {
	go doScan(m.state, m.ch)
	return tea.Batch(m.spinner.Tick, waitForActivity(m.ch))
}

func (m scanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, func() tea.Msg { return subCancelMsg{} }
		}

	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd

	case sourceStartMsg:
		m.current = msg.label
		return m, waitForActivity(m.ch)

	case sourceDoneMsg:
		m.stepsDone++
		m.completed = append(m.completed, completedEntry{label: msg.label, count: msg.count})
		pct := float64(m.stepsDone) / float64(m.totalSteps)
		return m, tea.Batch(m.progress.SetPercent(pct), waitForActivity(m.ch))

	case scanErrMsg:
		m.stepsDone++
		m.completed = append(m.completed, completedEntry{label: msg.label, isErr: true})
		pct := float64(m.stepsDone) / float64(m.totalSteps)
		return m, tea.Batch(m.progress.SetPercent(pct), waitForActivity(m.ch))

	case scanDoneMsg:
		m.allResults = msg.results
		m.done = true
		// Signal the parent model that scanning is complete.
		results := msg.results
		return m, func() tea.Msg { return scanFinishedMsg{results: results} }
	}

	return m, nil
}

func (m scanModel) View() tea.View {
	return tea.NewView(m.render())
}

func (m scanModel) render() string {
	if m.quitting {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")

	// Top spinner line
	if !m.done {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			m.spinner.View(),
			lipgloss.NewStyle().Faint(true).Render("Scanning: ")+
				lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")).Render(m.current),
		))
	}

	// Progress bar + step counter
	sb.WriteString("  ")
	if m.done {
		sb.WriteString(m.progress.ViewAs(1.0))
	} else {
		sb.WriteString(m.progress.View())
	}
	if m.totalSteps > 0 {
		sb.WriteString("  ")
		sb.WriteString(countStyle.Render(fmt.Sprintf("%d", m.stepsDone)))
		sb.WriteString(subtleStyle.Render("/"))
		sb.WriteString(countStyle.Render(fmt.Sprintf("%d", m.totalSteps)))
	}
	sb.WriteString("\n\n")

	// Completed sources list
	for _, c := range m.completed {
		if c.isErr {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				errorStyle.Render("✗"),
				errorStyle.Render(c.label),
			))
		} else {
			sb.WriteString(fmt.Sprintf("  %s  %-44s %s\n",
				successStyle.Render("✓"),
				c.label,
				subtleStyle.Render(fmt.Sprintf("(%d endpoints)", c.count)),
			))
		}
	}

	if m.done {
		sb.WriteString("\n")
	}

	return sb.String()
}

// doScan runs all source fetches sequentially, pushing progress messages into ch.
func doScan(state *appState, ch chan<- tea.Msg) {
	defer close(ch)
	var allResults [][]domain.EndpointTemplate

	for _, s := range state.waybackSources {
		label := fmt.Sprintf("wayback: %s%s", s.Domain, s.PathPrefix)
		ch <- sourceStartMsg{label: label}
		endpoints, err := domain.FetchWaybackURLs(context.Background(), s.Domain, s.PathPrefix)
		if err != nil {
			ch <- scanErrMsg{label: label, err: err}
			continue
		}
		allResults = append(allResults, endpoints)
		ch <- sourceDoneMsg{label: label, count: len(endpoints)}
	}

	for _, s := range state.logSources {
		label := fmt.Sprintf("logs: %s", s.FilePath)
		ch <- sourceStartMsg{label: label}
		endpoints, err := domain.ParseAccessLog(s.FilePath, s.Pattern)
		if err != nil {
			ch <- scanErrMsg{label: label, err: err}
			continue
		}
		allResults = append(allResults, endpoints)
		ch <- sourceDoneMsg{label: label, count: len(endpoints)}
	}

	for _, f := range state.swaggerSources {
		label := fmt.Sprintf("swagger: %s", f)
		ch <- sourceStartMsg{label: label}
		endpoints, err := domain.ParseSwaggerSpec(f)
		if err != nil {
			ch <- scanErrMsg{label: label, err: err}
			continue
		}
		allResults = append(allResults, endpoints)
		ch <- sourceDoneMsg{label: label, count: len(endpoints)}
	}

	ch <- scanDoneMsg{results: allResults}
}
