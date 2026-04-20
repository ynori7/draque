package domain

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// uuidRegexp matches a standard UUID (case-insensitive).
var uuidRegexp = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// excludedSegments are path segments that look numeric-like but are known static identifiers.
var excludedSegments = map[string]struct{}{
	"v1":  {},
	"v2":  {},
	"v3":  {},
	"v4":  {},
	"v5":  {},
	"v6":  {},
	"v7":  {},
	"v8":  {},
	"v9":  {},
	"b2b": {},
	"b2c": {},
	"b2p": {},
	"e2e": {},
	"p2p": {},
}

// isExcludedSegment reports whether a path segment should be treated as a static token
// rather than a dynamic parameter. It matches the segment exactly, or as a prefix/suffix,
// against the whitelisted patterns (comparison is case-insensitive).
func isExcludedSegment(seg string) bool {
	lower := strings.ToLower(seg)
	for key := range excludedSegments {
		if lower == key || strings.HasPrefix(lower, key) || strings.HasSuffix(lower, key) {
			return true
		}
	}
	return false
}

// NormalizeURL converts a raw absolute URL into a NormalizedPath (template + inferred parameters)
// and returns the normalized full URL string for observation storage.
func NormalizeURL(rawURL string) (NormalizedPath, string, error) {
	normalized, _, cleanedPath, ok := canonicalizeURL(rawURL)
	if !ok {
		return NormalizedPath{}, "", fmt.Errorf("invalid or unsupported URL: %q", rawURL)
	}
	if cleanedPath == "/" {
		return NormalizedPath{}, "", fmt.Errorf("URL has no meaningful path: %q", rawURL)
	}

	template, parameters := inferPathTemplate(cleanedPath)

	return NormalizedPath{
		Template:   template,
		Parameters: parameters,
	}, normalized, nil
}

// inferPathTemplate inspects each segment of a cleaned path, replaces dynamic segments with
// placeholders, and returns the resulting template string and ordered Parameter list.
func inferPathTemplate(cleanedPath string) (string, []Parameter) {
	segments := strings.Split(cleanedPath, "/")
	result := make([]string, 0, len(segments))
	parameters := make([]Parameter, 0)
	idCount := 0
	uuidCount := 0

	for _, seg := range segments {
		if seg == "" {
			result = append(result, seg)
			continue
		}

		if uuidRegexp.MatchString(seg) {
			uuidCount++
			name := "uuid"
			if uuidCount > 1 {
				name = fmt.Sprintf("uuid%d", uuidCount)
			}
			parameters = append(parameters, Parameter{
				Name:   name,
				Type:   "uuid",
				Source: "inferred",
			})
			result = append(result, "{"+name+"}")
			continue
		}

		if isExcludedSegment(seg) {
			result = append(result, seg)
			continue
		}

		if containsDigit(seg) {
			idCount++
			name := "id"
			if idCount > 1 {
				name = fmt.Sprintf("id%d", idCount)
			}
			parameters = append(parameters, Parameter{
				Name:   name,
				Type:   "int",
				Source: "inferred",
			})
			result = append(result, "{"+name+"}")
			continue
		}

		result = append(result, seg)
	}

	return strings.Join(result, "/"), parameters
}

// canonicalizeURL normalizes scheme, host, port, path, and query parameters for a raw URL.
// Returns (normalizedURL, host, cleanedPath, ok). Returns false for invalid or non-http(s) URLs.
func canonicalizeURL(rawURL string) (string, string, string, bool) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || !parsedURL.IsAbs() {
		return "", "", "", false
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", "", false
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "" {
		return "", "", "", false
	}

	port := parsedURL.Port()
	if scheme == "http" && port == "80" {
		port = ""
	}
	if scheme == "https" && port == "443" {
		port = ""
	}

	host := joinHostPort(hostname, port)
	parsedURL.Scheme = scheme
	parsedURL.Host = host
	parsedURL.Fragment = ""

	cleanedPath := path.Clean(parsedURL.EscapedPath())
	if cleanedPath == "." {
		cleanedPath = "/"
	}
	if !strings.HasPrefix(cleanedPath, "/") {
		cleanedPath = "/" + cleanedPath
	}

	sortedQuery := parsedURL.Query().Encode()
	parsedURL.Path = cleanedPath
	parsedURL.RawPath = ""
	parsedURL.RawQuery = sortedQuery

	return parsedURL.String(), host, cleanedPath, true
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}
