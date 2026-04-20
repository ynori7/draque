package draque

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ynori7/draque/domain"
)

// Draque is the main library entry point. Configure it with functional options
// via New, then call Scan to process all configured sources and return merged
// endpoint templates.
type Draque struct {
	waybackSources []waybackSource
	logSources     []logSource
	swaggerSources []string
	errorOnFailure bool
	configErr      error
	limits         domain.ScanLimits
}

type waybackSource struct {
	domain     string
	pathPrefix string
}

type logSource struct {
	filePath string
	pattern  string
}

// Option is a functional option for configuring a Draque instance.
type Option func(*Draque)

// New creates a new Draque instance with the given options applied.
func New(opts ...Option) *Draque {
	d := &Draque{}
	for _, opt := range opts {
		opt(d)
		if d.configErr != nil {
			break
		}
	}
	return d
}

// WithWayback adds a Wayback Machine source. url is the domain (e.g. "example.com")
// and prefix is an optional URL path prefix to restrict results (e.g. "/api").
func WithWayback(url, prefix string) Option {
	return func(d *Draque) {
		d.waybackSources = append(d.waybackSources, waybackSource{domain: url, pathPrefix: prefix})
	}
}

// WithLogFile adds a single access log file as a source. path is the file path
// and format is the log pattern string (e.g. "{host} {method} {path} {status}").
func WithLogFile(path, format string) Option {
	return func(d *Draque) {
		d.logSources = append(d.logSources, logSource{filePath: path, pattern: format})
	}
}

// WithLogDirectory adds all regular files in the given directory as log sources,
// each parsed with the same format pattern.
func WithLogDirectory(path, format string) Option {
	return func(d *Draque) {
		entries, err := os.ReadDir(path)
		if err != nil {
			d.configErr = fmt.Errorf("WithLogDirectory: reading directory %q: %w", path, err)
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			d.logSources = append(d.logSources, logSource{
				filePath: filepath.Join(path, entry.Name()),
				pattern:  format,
			})
		}
	}
}

// WithSwagger adds a single Swagger/OpenAPI spec file as a source.
// Supports JSON and YAML formats (OpenAPI v2 and v3).
func WithSwagger(path string) Option {
	return func(d *Draque) {
		d.swaggerSources = append(d.swaggerSources, path)
	}
}

// WithSwaggerDirectory adds all Swagger/OpenAPI spec files (.json, .yaml, .yml)
// in the given directory as sources.
func WithSwaggerDirectory(path string) Option {
	return func(d *Draque) {
		entries, err := os.ReadDir(path)
		if err != nil {
			d.configErr = fmt.Errorf("WithSwaggerDirectory: reading directory %q: %w", path, err)
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			switch filepath.Ext(entry.Name()) {
			case ".json", ".yaml", ".yml":
				d.swaggerSources = append(d.swaggerSources, filepath.Join(path, entry.Name()))
			}
		}
	}
}

// WithErrorOnFailure controls error handling in Scan.
// When true, Scan returns immediately on the first source failure.
// When false (the default), failed sources are skipped and scanning continues.
func WithErrorOnFailure(v bool) Option {
	return func(d *Draque) {
		d.errorOnFailure = v
	}
}

// WithMaxObservations caps the number of observation URLs stored per endpoint.
// This bounds memory usage when processing large log files.
// A value of zero (the default) means no limit.
func WithMaxObservations(n int) Option {
	return func(d *Draque) {
		d.limits.MaxObservations = n
	}
}

// WithMaxExamples caps the number of example parameter values stored per endpoint.
// A value of zero (the default) means no limit.
func WithMaxExamples(n int) Option {
	return func(d *Draque) {
		d.limits.MaxExamples = n
	}
}

// Scan processes all configured sources in order (Wayback, logs, Swagger)
// and returns the merged, deduplicated set of endpoint templates.
//
// If WithErrorOnFailure(true) was set, the first source failure causes Scan to
// return immediately with that error. Otherwise, failed sources are skipped.
func (d *Draque) Scan() ([]domain.EndpointTemplate, error) {
	if d.configErr != nil {
		return nil, d.configErr
	}

	var allResults [][]domain.EndpointTemplate

	for _, s := range d.waybackSources {
		endpoints, err := domain.FetchWaybackURLs(context.Background(), s.domain, s.pathPrefix, d.limits)
		if err != nil {
			if d.errorOnFailure {
				return nil, fmt.Errorf("wayback %q: %w", s.domain, err)
			}
			continue
		}
		allResults = append(allResults, endpoints)
	}

	for _, s := range d.logSources {
		compiled, err := domain.CompileAccessLogPattern(s.pattern)
		if err != nil {
			if d.errorOnFailure {
				return nil, fmt.Errorf("log file %q: %w", s.filePath, err)
			}
			continue
		}
		endpoints, err := domain.ParseAccessLogWithPattern(s.filePath, compiled, d.limits)
		if err != nil {
			if d.errorOnFailure {
				return nil, fmt.Errorf("log file %q: %w", s.filePath, err)
			}
			continue
		}
		allResults = append(allResults, endpoints)
	}

	for _, f := range d.swaggerSources {
		endpoints, err := domain.ParseSwaggerSpec(f)
		if err != nil {
			if d.errorOnFailure {
				return nil, fmt.Errorf("swagger %q: %w", f, err)
			}
			continue
		}
		allResults = append(allResults, endpoints)
	}

	if len(allResults) == 0 {
		return nil, nil
	}

	return domain.MatchTemplatesWithLimits(d.limits, allResults...), nil
}
