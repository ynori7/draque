package domain

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var logPatternTokenRegexp = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// AccessLogPattern is a compiled parser for a log format pattern.
type AccessLogPattern struct {
	re        *regexp.Regexp
	hasMethod bool
	hasStatus bool
}

// CompileAccessLogPattern compiles a user format string into an efficient line parser.
// The pattern supports named placeholders. We're interested in these particular
// placeholders: {path}, {method}, {host}, {status}.
func CompileAccessLogPattern(pattern string) (*AccessLogPattern, error) {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return nil, fmt.Errorf("log pattern is required")
	}

	matches := logPatternTokenRegexp.FindAllStringSubmatchIndex(trimmed, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("log pattern must contain placeholders")
	}

	var builder strings.Builder
	builder.WriteString("^")

	last := 0
	hasHost := false
	hasPath := false
	hasMethod := false
	hasStatus := false

	for _, m := range matches {
		builder.WriteString(regexp.QuoteMeta(trimmed[last:m[0]]))

		name := strings.ToLower(trimmed[m[2]:m[3]])
		switch name {
		case "host":
			if hasHost {
				return nil, fmt.Errorf("log pattern contains duplicate {host} placeholder")
			}
			hasHost = true
			builder.WriteString(`(?P<host>\S+?)`)
		case "path":
			if hasPath {
				return nil, fmt.Errorf("log pattern contains duplicate {path} placeholder")
			}
			hasPath = true
			builder.WriteString(`(?P<path>(?:https?://\S+|/\S*|\?\S+))`)
		case "method":
			if hasMethod {
				return nil, fmt.Errorf("log pattern contains duplicate {method} placeholder")
			}
			hasMethod = true
			builder.WriteString(`(?P<method>\S+)`)
		case "status":
			if hasStatus {
				return nil, fmt.Errorf("log pattern contains duplicate {status} placeholder")
			}
			hasStatus = true
			builder.WriteString(`(?P<status>[0-9]{3})`)
		default:
			builder.WriteString(`.+?`)
		}

		last = m[1]
	}

	builder.WriteString(regexp.QuoteMeta(trimmed[last:]))
	builder.WriteString("$")

	if !hasHost || !hasPath {
		return nil, fmt.Errorf("log pattern must include {host} and {path}")
	}

	re, err := regexp.Compile(builder.String())
	if err != nil {
		return nil, fmt.Errorf("compile log pattern: %w", err)
	}

	return &AccessLogPattern{re: re, hasMethod: hasMethod, hasStatus: hasStatus}, nil
}

// ParseAccessLog parses one log file into endpoint templates using the given pattern.
func ParseAccessLog(filePath string, pattern string) ([]EndpointTemplate, error) {
	compiledPattern, err := CompileAccessLogPattern(pattern)
	if err != nil {
		return nil, err
	}

	return ParseAccessLogWithPattern(filePath, compiledPattern)
}

// ParseAccessLogWithPattern parses one log file with a pre-compiled pattern.
func ParseAccessLogWithPattern(filePath string, pattern *AccessLogPattern) ([]EndpointTemplate, error) {
	if pattern == nil || pattern.re == nil {
		return nil, fmt.Errorf("compiled log pattern is required")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	observations := make([]endpointObservation, 0)
	for scanner.Scan() {
		method, host, requestPath, statusCode, ok := pattern.parseLine(scanner.Text())
		if !ok {
			continue
		}

		if pattern.hasStatus && statusCode == "404" {
			continue
		}

		rawURL := buildLogURL(host, requestPath)
		if rawURL == "" {
			continue
		}

		_, normalizedURL, err := NormalizeURL(rawURL)
		if err != nil {
			continue
		}

		var statusInt int
		if sc, err := strconv.Atoi(statusCode); err == nil {
			statusInt = sc
		}

		observations = append(observations, endpointObservation{
			URL:        normalizedURL,
			Method:     method,
			StatusCode: statusInt,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log file: %w", err)
	}

	return aggregateEndpoints(observations, "logs"), nil
}

// TryParseLine reports whether the given log line matches this pattern.
func (p *AccessLogPattern) TryParseLine(line string) bool {
	_, _, _, _, ok := p.parseLine(line)
	return ok
}

func (p *AccessLogPattern) parseLine(line string) (string, string, string, string, bool) {
	matches := p.re.FindStringSubmatch(line)
	if matches == nil {
		return "", "", "", "", false
	}

	host := strings.TrimSpace(p.capture(matches, "host"))
	requestPath := strings.TrimSpace(p.capture(matches, "path"))
	if host == "" || requestPath == "" {
		return "", "", "", "", false
	}

	method := strings.ToUpper(strings.TrimSpace(p.capture(matches, "method")))
	if method == "" {
		method = "GET"
	}

	statusCode := strings.TrimSpace(p.capture(matches, "status"))
	if statusCode == "" {
		statusCode = "200"
	}

	return method, host, requestPath, statusCode, true
}

func (p *AccessLogPattern) capture(matches []string, name string) string {
	for index, groupName := range p.re.SubexpNames() {
		if groupName == name && index < len(matches) {
			return matches[index]
		}
	}

	return ""
}

func buildLogURL(host string, requestPath string) string {
	h := strings.TrimSpace(host)
	p := strings.TrimSpace(requestPath)
	if p == "" {
		return ""
	}

	if strings.HasSuffix(h, ":") && strings.HasPrefix(p, "//") {
		return h + p
	}

	lowerPath := strings.ToLower(p)
	if strings.HasPrefix(lowerPath, "http://") || strings.HasPrefix(lowerPath, "https://") {
		return p
	}

	if h == "" {
		return ""
	}

	base := h
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}

	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "?") {
		return strings.TrimRight(base, "/") + p
	}

	return strings.TrimRight(base, "/") + "/" + p
}
