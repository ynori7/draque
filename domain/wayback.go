package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const DefaultWaybackCDXEndpoint = "https://web.archive.org/cdx/search/cdx"

// common static file types that aren't interesting to look at
var staticAssetExtensions = map[string]struct{}{
	".css":   {},
	".jpg":   {},
	".jpeg":  {},
	".js":    {},
	".png":   {},
	".svg":   {},
	".webp":  {},
	".gif":   {},
	".ico":   {},
	".ttf":   {},
	".woff":  {},
	".woff2": {},
}

// WaybackFetcher loads archived URLs from a CDX endpoint.
type WaybackFetcher struct {
	BaseURL    string
	HTTPClient *http.Client
	PageSize   int
}

// FetchWaybackURLs loads archived URLs from the public Internet Archive CDX API and
// aggregates them into endpoint templates grouped by normalized path template.
// Observations per endpoint are capped to limits.MaxObservations when non-zero.
func FetchWaybackURLs(ctx context.Context, domain string, pathPrefix string, limits ScanLimits) ([]EndpointTemplate, error) {
	fetcher := WaybackFetcher{
		BaseURL: DefaultWaybackCDXEndpoint,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	records, err := fetcher.FetchURLs(ctx, domain, pathPrefix)
	if err != nil {
		return nil, err
	}

	return aggregateEndpoints(urlRecordsToObservations(records), "wayback", limits), nil
}

// urlRecord is a lightweight pair of (normalized URL, HTTP status code string) returned
// by FetchURLs. StatusCode is the raw string from the CDX response and may be empty.
type urlRecord struct {
	URL        string
	StatusCode string
}

func urlRecordsToObservations(records []urlRecord) []endpointObservation {
	observations := make([]endpointObservation, 0, len(records))
	for _, r := range records {
		var sc int
		if n, err := strconv.Atoi(r.StatusCode); err == nil {
			sc = n
		}
		observations = append(observations, endpointObservation{URL: r.URL, StatusCode: sc})
	}
	return observations
}

// aggregateEndpoints converts a list of normalized URLs into endpoint templates by
// grouping on the normalized path template and collecting observations.
// Observations per endpoint are capped to limits.MaxObservations when non-zero.
func aggregateEndpoints(observations []endpointObservation, source string, limits ScanLimits) []EndpointTemplate {
	byTemplate := make(map[string]*EndpointTemplate, len(observations))
	order := make([]string, 0, len(observations))
	type obsKey struct {
		URL        string
		StatusCode int
	}
	seenObs := make(map[string]map[obsKey]struct{}, len(observations))

	for _, observation := range observations {
		normalizedURL := observation.URL
		np, _, err := NormalizeURL(normalizedURL)
		if err != nil {
			continue
		}

		method := strings.ToUpper(strings.TrimSpace(observation.Method))

		key := np.Template
		if method != "" {
			key = method + "\x00" + np.Template
		}

		et, exists := byTemplate[key]
		if !exists {
			et = &EndpointTemplate{
				Method:       method,
				PathTemplate: np.Template,
				Parameters:   np.Parameters,
			}
			byTemplate[key] = et
			order = append(order, key)
		}

		et.Count++
		if limits.MaxObservations == 0 || len(et.Observations) < limits.MaxObservations {
			ok := obsKey{normalizedURL, observation.StatusCode}
			if seenObs[key] == nil {
				cap := limits.MaxObservations
				if cap == 0 {
					cap = 16
				}
				seenObs[key] = make(map[obsKey]struct{}, cap)
			}
			if _, dup := seenObs[key][ok]; !dup {
				et.Observations = append(et.Observations, ExampleURL{Source: source, URL: normalizedURL, StatusCode: observation.StatusCode})
				seenObs[key][ok] = struct{}{}
			}
		}
	}

	result := make([]EndpointTemplate, 0, len(order))
	for _, tmpl := range order {
		result = append(result, *byTemplate[tmpl])
	}

	return result
}

// FetchURLs loads archived URLs for the provided domain and optional path prefix.
func (f WaybackFetcher) FetchURLs(ctx context.Context, domain string, pathPrefix string) ([]urlRecord, error) {
	target, normalizedPrefix, err := buildWaybackQueryTarget(domain, pathPrefix)
	if err != nil {
		return nil, err
	}

	pageCount, paginationSupported, err := f.lookupPageCount(ctx, target)
	if err != nil {
		return nil, err
	}

	pages := []int{-1}
	if paginationSupported {
		if pageCount == 0 {
			return []urlRecord{}, nil
		}

		pages = make([]int, 0, pageCount)
		for pageIndex := 0; pageIndex < pageCount; pageIndex++ {
			pages = append(pages, pageIndex)
		}
	}

	results := make([]urlRecord, 0)
	seenPaths := make(map[string]struct{})

	for _, pageIndex := range pages {
		records, err := f.fetchPage(ctx, target, pageIndex)
		if err != nil {
			return nil, err
		}

		for _, record := range records {
			/*
			 * Skip results with a 404 status code since the endpoint is probably non-existent,
			 * if the URL is invalid or points to a static asset, if it doesn't match the given path prefix,
			 * or if we've already seen the same normalized path from a previous page of results.
			 */
			if record.StatusCode == "404" {
				continue
			}

			normalizedURL, dedupePath, normalizedPath, ok := normalizeWaybackResultURL(record.Original)
			if !ok {
				continue
			}

			if !matchesPathPrefix(normalizedPath, normalizedPrefix) {
				continue
			}

			if _, skip := seenPaths[dedupePath]; skip {
				continue
			}

			seenPaths[dedupePath] = struct{}{}
			results = append(results, urlRecord{URL: normalizedURL, StatusCode: record.StatusCode})
		}
	}

	return results, nil
}

type cdxRecord struct {
	Original   string
	StatusCode string
}

func (f WaybackFetcher) fetchPage(ctx context.Context, target string, pageIndex int) ([]cdxRecord, error) {
	requestURL, err := f.buildCDXRequestURL(target, pageIndex, false)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create CDX request: %w", err)
	}

	response, err := f.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("request CDX page: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("CDX request failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	return decodeCDXJSON(response.Body)
}

func (f WaybackFetcher) lookupPageCount(ctx context.Context, target string) (int, bool, error) {
	requestURL, err := f.buildCDXRequestURL(target, -1, true)
	if err != nil {
		return 0, false, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return 0, false, fmt.Errorf("create pagination request: %w", err)
	}

	response, err := f.httpClient().Do(request)
	if err != nil {
		return 0, false, fmt.Errorf("request CDX pagination info: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusBadRequest {
		return 0, false, nil
	}

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return 0, false, fmt.Errorf("CDX pagination request failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, false, fmt.Errorf("read pagination response: %w", err)
	}

	pageCount, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil {
		return 0, false, fmt.Errorf("parse pagination response %q: %w", strings.TrimSpace(string(body)), err)
	}

	return pageCount, true, nil
}

func (f WaybackFetcher) buildCDXRequestURL(target string, pageIndex int, showNumPages bool) (string, error) {
	baseURL := f.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultWaybackCDXEndpoint
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base CDX URL: %w", err)
	}

	query := parsedURL.Query()
	query.Set("url", target)
	query.Set("matchType", "domain")

	if f.PageSize > 0 {
		query.Set("pageSize", strconv.Itoa(f.PageSize))
	}

	if showNumPages {
		query.Set("showNumPages", "true")
		parsedURL.RawQuery = query.Encode()
		return parsedURL.String(), nil
	}

	query.Set("output", "json")
	query.Set("fl", "original,statuscode")
	if pageIndex >= 0 {
		query.Set("page", strconv.Itoa(pageIndex))
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func (f WaybackFetcher) httpClient() *http.Client {
	if f.HTTPClient != nil {
		return f.HTTPClient
	}

	return http.DefaultClient
}

func decodeCDXJSON(reader io.Reader) ([]cdxRecord, error) {
	var rows [][]string
	if err := json.NewDecoder(reader).Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode CDX JSON: %w", err)
	}

	if len(rows) == 0 {
		return []cdxRecord{}, nil
	}

	fieldIndexes := make(map[string]int, len(rows[0]))
	for index, field := range rows[0] {
		fieldIndexes[field] = index
	}

	originalIndex, ok := fieldIndexes["original"]
	if !ok {
		return nil, fmt.Errorf("CDX JSON missing original field")
	}

	statusIndex, ok := fieldIndexes["statuscode"]
	if !ok {
		return nil, fmt.Errorf("CDX JSON missing statuscode field")
	}

	records := make([]cdxRecord, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) <= originalIndex || len(row) <= statusIndex {
			continue
		}

		records = append(records, cdxRecord{
			Original:   row[originalIndex],
			StatusCode: row[statusIndex],
		})
	}

	return records, nil
}

func buildWaybackQueryTarget(domain string, pathPrefix string) (string, string, error) {
	normalizedDomain, err := normalizeDomain(domain)
	if err != nil {
		return "", "", err
	}

	normalizedPrefix := normalizePrefix(pathPrefix)
	if normalizedPrefix == "" {
		return normalizedDomain + "/", normalizedPrefix, nil
	}

	return normalizedDomain + normalizedPrefix, normalizedPrefix, nil
}

// ValidateWaybackDomain reports whether domain is a valid hostname or URL that
// can be used as a Wayback Machine scan target. It applies the same normalisation
// rules used by FetchWaybackURLs.
func ValidateWaybackDomain(domain string) error {
	_, err := normalizeDomain(domain)
	return err
}

func normalizeDomain(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("domain is required")
	}

	if strings.Contains(trimmed, "://") {
		parsedURL, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse domain %q: %w", input, err)
		}
		if parsedURL.Host == "" {
			return "", fmt.Errorf("domain %q is invalid", input)
		}
		trimmed = parsedURL.Host
	}

	if slashIndex := strings.Index(trimmed, "/"); slashIndex >= 0 {
		trimmed = trimmed[:slashIndex]
	}

	parsedURL, err := url.Parse("//" + trimmed)
	if err != nil {
		return "", fmt.Errorf("parse domain %q: %w", input, err)
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "" {
		return "", fmt.Errorf("domain %q is invalid", input)
	}

	port := parsedURL.Port()
	return joinHostPort(hostname, port), nil
}

func normalizePrefix(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed == "/" {
		return ""
	}

	if strings.Contains(trimmed, "://") {
		if parsedURL, err := url.Parse(trimmed); err == nil {
			trimmed = parsedURL.Path
		}
	}

	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}

	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == "/" {
		return ""
	}

	return strings.TrimRight(cleaned, "/")
}

// normalizeWaybackResultURL normalizes a URL by lowercasing the scheme and hostname, removing default ports, cleaning the path,
// sorting query parameters, and excluding static asset paths. It returns the normalized URL, a path-only dedupe key, the cleaned
// path, and a boolean indicating whether the URL is valid for consideration.
//
// Returns the normalized URL, a path-only dedupe key, the cleaned path, and a boolean indicating whether the URL is valid for
// consideration. Returns false if the URL is invalid or should be excluded.
func normalizeWaybackResultURL(rawURL string) (string, string, string, bool) {
	normalizedURL, host, cleanedPath, ok := canonicalizeURL(rawURL)
	if !ok {
		return "", "", "", false
	}

	if cleanedPath == "/" {
		return "", "", "", false
	}

	if isStaticAssetPath(cleanedPath) {
		return "", "", "", false
	}

	dedupePath := host + cleanedPath
	return normalizedURL, dedupePath, cleanedPath, true
}

func matchesPathPrefix(candidate string, prefix string) bool {
	if prefix == "" {
		return true
	}

	if candidate == prefix {
		return true
	}

	return strings.HasPrefix(candidate, prefix+"/")
}

func isStaticAssetPath(urlPath string) bool {
	extension := strings.ToLower(path.Ext(urlPath))
	_, excluded := staticAssetExtensions[extension]
	return excluded
}

func joinHostPort(hostname string, port string) string {
	if port == "" {
		if strings.Contains(hostname, ":") {
			return "[" + hostname + "]"
		}
		return hostname
	}

	return net.JoinHostPort(hostname, port)
}
