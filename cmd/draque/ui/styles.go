package ui

import "charm.land/lipgloss/v2"

var (
	// Layout / structural styles
	bannerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")) // bright cyan-green
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")) // bright green
	subtleStyle = lipgloss.NewStyle().Faint(true)

	// Status / feedback
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))             // green
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")) // red
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))            // orange-yellow

	// Section headers inside commands
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")) // bright white

	// Per-source label colors
	waybackStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))  // blue
	logsStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")) // yellow
	swaggerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("83"))  // green

	// Data highlights
	methodStyle = lipgloss.NewStyle().Bold(true)
	countStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")) // yellow
	paramStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))            // magenta-pink
	pathStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))            // light grey

	// Status code colors
	status2xxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))  // green
	status4xxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // orange
	status5xxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	statusDimStyle = lipgloss.NewStyle().Faint(true)

	// Help formatting
	cmdNameStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	cmdDescStyle = lipgloss.NewStyle().Faint(true)

	// Sub-model text inputs
	inputTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // light grey

	// Search UI
	cursorStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	selectedStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("236"))
	matchCountStyle = lipgloss.NewStyle().Faint(true).Italic(true)
)

// sourceStyle returns the per-source lipgloss style for a source name.
func sourceStyle(src string) lipgloss.Style {
	switch src {
	case "wayback":
		return waybackStyle
	case "logs":
		return logsStyle
	case "swagger":
		return swaggerStyle
	default:
		return lipgloss.NewStyle()
	}
}

// methodColor returns the lipgloss style for an HTTP method.
func methodColor(method string) lipgloss.Style {
	switch method {
	case "GET":
		return methodStyle.Foreground(lipgloss.Color("39")) // blue
	case "POST":
		return methodStyle.Foreground(lipgloss.Color("82")) // green
	case "PUT":
		return methodStyle.Foreground(lipgloss.Color("220")) // yellow
	case "PATCH":
		return methodStyle.Foreground(lipgloss.Color("214")) // orange
	case "DELETE":
		return methodStyle.Foreground(lipgloss.Color("196")) // red
	default:
		return methodStyle.Foreground(lipgloss.Color("252")) // grey
	}
}

// statusCodeStyle returns the lipgloss style for an HTTP status code.
func statusCodeStyle(code int) lipgloss.Style {
	switch {
	case code == 0:
		return statusDimStyle
	case code >= 200 && code < 300:
		return status2xxStyle
	case code >= 400 && code < 500:
		return status4xxStyle
	case code >= 500:
		return status5xxStyle
	default:
		return lipgloss.NewStyle()
	}
}
