package main

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
