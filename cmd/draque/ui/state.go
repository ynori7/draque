package ui

import "github.com/ynori7/draque/domain"

type waybackSource struct {
	Domain     string
	PathPrefix string
}

type logSource struct {
	FilePath string
	Pattern  string
}

type appState struct {
	waybackSources []waybackSource
	logSources     []logSource
	swaggerSources []string
	results        []domain.EndpointTemplate
	scanned        bool
}

// Reset clears all configured sources and scan data, returning to an empty state.
func (s *appState) Reset() {
	s.waybackSources = nil
	s.logSources = nil
	s.swaggerSources = nil
	s.results = nil
	s.scanned = false
}
