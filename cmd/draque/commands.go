package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ynori7/draque/domain"
)

// ---- input helpers ----

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptRequired(reader *bufio.Reader, label string) string {
	for {
		v := prompt(reader, label)
		if v != "" {
			return v
		}
		fmt.Println("  This field is required.")
	}
}

// ---- command: wayback ----

func cmdWayback(reader *bufio.Reader, state *appState) {
	var domainInput string
	for {
		domainInput = promptRequired(reader, "  Domain (e.g. example.com): ")
		if err := domain.ValidateWaybackDomain(domainInput); err != nil {
			fmt.Printf("  Invalid domain: %v\n", err)
			continue
		}
		break
	}
	prefix := prompt(reader, "  Path prefix (optional, press Enter to skip): ")
	state.waybackSources = append(state.waybackSources, waybackSource{Domain: domainInput, PathPrefix: prefix})
	fmt.Printf("  Added wayback source: %s%s\n", domainInput, prefix)
}

// ---- command: logs ----

func cmdLogs(reader *bufio.Reader, state *appState) {
	// 1. File path — validate existence before asking for pattern.
	var filePath string
	for {
		filePath = promptRequired(reader, "  Log file path: ")
		if _, err := os.Stat(filePath); err != nil {
			fmt.Printf("  File not found: %s\n", filePath)
			continue
		}
		break
	}

	// 2. Pattern selection + first-line validation (retry on mismatch).
	for {
		pattern := selectOrEnterPattern(reader, state)
		compiled, err := domain.CompileAccessLogPattern(pattern)
		if err != nil {
			fmt.Printf("  Invalid pattern: %v\n", err)
			continue
		}
		if err := validateFirstLogLine(filePath, compiled); err != nil {
			fmt.Printf("  %v\n", err)
			fmt.Println("  Please re-enter or re-select the log format pattern.")
			continue
		}
		state.logSources = append(state.logSources, logSource{FilePath: filePath, Pattern: pattern})
		fmt.Printf("  Added log source: %s\n", filePath)
		return
	}
}

// selectOrEnterPattern presents a reuse menu when previous log sources exist,
// otherwise prompts for a new pattern string, validating it compiles.
func selectOrEnterPattern(reader *bufio.Reader, state *appState) string {
	if len(state.logSources) > 0 {
		fmt.Println("  Reuse a format from a previous log source?")
		fmt.Println("  [0] Enter a new format")
		for i, s := range state.logSources {
			fmt.Printf("  [%d] %s\n", i+1, s.Pattern)
		}
		choice := prompt(reader, "  Choice [0]: ")
		if choice != "" && choice != "0" {
			if idx, err := strconv.Atoi(choice); err == nil && idx >= 1 && idx <= len(state.logSources) {
				return state.logSources[idx-1].Pattern
			}
		}
	}
	for {
		p := promptRequired(reader, "  Log format pattern (use {host}, {path}, {method}, {status}, ...): ")
		if _, err := domain.CompileAccessLogPattern(p); err != nil {
			fmt.Printf("  Invalid pattern: %v\n", err)
			continue
		}
		return p
	}
}

// validateFirstLogLine opens the file and checks whether any of the first five
// non-empty lines can be parsed by the compiled pattern. Returns an error if none
// match, so the caller can ask the user to correct the pattern.
func validateFirstLogLine(filePath string, pattern *domain.AccessLogPattern) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	checked := 0
	for scanner.Scan() && checked < 5 {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		checked++
		if pattern.TryParseLine(line) {
			return nil
		}
	}
	if checked == 0 {
		// Empty file — no lines to validate against; allow proceeding.
		return nil
	}
	return fmt.Errorf("the log format pattern did not match the first %d line(s) of %s", checked, filePath)
}

// ---- command: swagger ----

func cmdSwagger(reader *bufio.Reader, state *appState) {
	var filePath string
	for {
		filePath = promptRequired(reader, "  Swagger/OpenAPI spec file path: ")
		if _, err := os.Stat(filePath); err != nil {
			fmt.Printf("  File not found: %s\n", filePath)
			continue
		}
		break
	}
	state.swaggerSources = append(state.swaggerSources, filePath)
	fmt.Printf("  Added swagger source: %s\n", filePath)
}

// ---- command: scan ----

func cmdScan(state *appState) {
	if len(state.waybackSources) == 0 && len(state.logSources) == 0 && len(state.swaggerSources) == 0 {
		fmt.Println("  No sources configured. Use 'wayback', 'logs', or 'swagger' to add sources first.")
		return
	}

	var allSources [][]domain.EndpointTemplate

	for _, s := range state.waybackSources {
		fmt.Printf("  Fetching wayback URLs for %s%s ...\n", s.Domain, s.PathPrefix)
		endpoints, err := domain.FetchWaybackURLs(context.Background(), s.Domain, s.PathPrefix)
		if err != nil {
			fmt.Printf("  Error fetching wayback URLs for %s: %v\n", s.Domain, err)
			continue
		}
		fmt.Printf("  Found %d endpoint templates from wayback (%s).\n", len(endpoints), s.Domain)
		allSources = append(allSources, endpoints)
	}

	for _, s := range state.logSources {
		fmt.Printf("  Parsing log file %s ...\n", s.FilePath)
		endpoints, err := domain.ParseAccessLog(s.FilePath, s.Pattern)
		if err != nil {
			fmt.Printf("  Error parsing log file %s: %v\n", s.FilePath, err)
			continue
		}
		fmt.Printf("  Found %d endpoint templates from logs (%s).\n", len(endpoints), s.FilePath)
		allSources = append(allSources, endpoints)
	}

	for _, f := range state.swaggerSources {
		fmt.Printf("  Parsing swagger spec %s ...\n", f)
		endpoints, err := domain.ParseSwaggerSpec(f)
		if err != nil {
			fmt.Printf("  Error parsing swagger file %s: %v\n", f, err)
			continue
		}
		fmt.Printf("  Found %d endpoint templates from swagger (%s).\n", len(endpoints), f)
		allSources = append(allSources, endpoints)
	}

	if len(allSources) == 0 {
		fmt.Println("  All sources failed. No results.")
		return
	}

	state.results = domain.MatchTemplates(allSources...)
	state.scanned = true
	fmt.Printf("\n  Scan complete. %d unique endpoint templates.\n", len(state.results))
}

// ---- command: analyze ----

func cmdAnalyze(state *appState) {
	if !state.scanned {
		fmt.Println("  No scan results yet. Run 'scan' first.")
		return
	}

	totalCount := 0
	sourceCounts := map[string]int{}
	for _, ep := range state.results {
		totalCount += ep.Count
		for _, obs := range ep.Observations {
			sourceCounts[obs.Source]++
		}
	}

	fmt.Printf("\n  Total unique endpoint templates : %d\n", len(state.results))
	fmt.Printf("  Total observations              : %d\n", totalCount)
	if len(sourceCounts) > 0 {
		fmt.Println("\n  Observations by source:")
		for _, src := range []string{"wayback", "logs", "swagger"} {
			if n, ok := sourceCounts[src]; ok {
				fmt.Printf("    %-10s %d\n", src, n)
			}
		}
	}

	sorted := make([]domain.EndpointTemplate, len(state.results))
	copy(sorted, state.results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	top := sorted
	if len(top) > 10 {
		top = top[:10]
	}
	fmt.Println("\n  Top endpoints by observation count:")
	for i, ep := range top {
		fmt.Printf("    %2d. %-6s %-50s  count: %d\n", i+1, ep.Method, ep.PathTemplate, ep.Count)
	}
}

// ---- command: search ----

func cmdSearch(reader *bufio.Reader, state *appState) {
	if !state.scanned {
		fmt.Println("  No scan results yet. Run 'scan' first.")
		return
	}

	prefix := promptRequired(reader, "  Search prefix (e.g. /api/users): ")

	var matches []domain.EndpointTemplate
	lowerPrefix := strings.ToLower(prefix)
	for _, ep := range state.results {
		if strings.HasPrefix(strings.ToLower(ep.PathTemplate), lowerPrefix) {
			matches = append(matches, ep)
		}
	}

	if len(matches) == 0 {
		fmt.Println("  No endpoints match that prefix.")
		return
	}

	fmt.Printf("\n  Found %d matching endpoint(s):\n", len(matches))
	for i, ep := range matches {
		fmt.Printf("  [%d] %-6s %s  (count: %d)\n", i+1, ep.Method, ep.PathTemplate, ep.Count)
	}

	choiceStr := promptRequired(reader, "\n  Select endpoint number (or 0 to cancel): ")
	choice, err := strconv.Atoi(choiceStr)
	if err != nil || choice < 1 || choice > len(matches) {
		if choiceStr != "0" {
			fmt.Println("  Invalid selection.")
		}
		return
	}

	printEndpointDetail(matches[choice-1])
}

func printEndpointDetail(ep domain.EndpointTemplate) {
	fmt.Printf("\n  ---- %s %s ----\n", ep.Method, ep.PathTemplate)
	fmt.Printf("  Count: %d\n", ep.Count)

	// Sources
	sourceSet := map[string]struct{}{}
	for _, obs := range ep.Observations {
		sourceSet[obs.Source] = struct{}{}
	}
	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	fmt.Printf("  Sources: %s\n", strings.Join(sources, ", "))

	// Parameters
	if len(ep.Parameters) > 0 {
		fmt.Println("\n  Parameters:")
		for _, p := range ep.Parameters {
			fmt.Printf("    {%s}  type: %-10s  source: %s\n", p.Name, p.Type, p.Source)
		}
	}

	// Example parameter values
	if len(ep.Examples) > 0 {
		fmt.Println("\n  Observed parameter values:")
		// Group by param name
		byParam := map[string][]domain.ExampleParameter{}
		order := []string{}
		for _, ex := range ep.Examples {
			if _, seen := byParam[ex.ParamName]; !seen {
				order = append(order, ex.ParamName)
			}
			byParam[ex.ParamName] = append(byParam[ex.ParamName], ex)
		}
		for _, name := range order {
			vals := byParam[name]
			strs := make([]string, 0, len(vals))
			for _, v := range vals {
				strs = append(strs, fmt.Sprintf("%s (from %s)", v.Value, v.Source))
			}
			fmt.Printf("    {%s}: %s\n", name, strings.Join(strs, ", "))
		}
	}

	// Observed URLs
	if len(ep.Observations) > 0 {
		fmt.Println("\n  Observed URLs:")
		for _, obs := range ep.Observations {
			statusStr := ""
			if obs.StatusCode != 0 {
				statusStr = fmt.Sprintf(" [%d]", obs.StatusCode)
			}
			fmt.Printf("    [%-7s]%s %s\n", obs.Source, statusStr, obs.URL)
		}
	}
}

// ---- command: export ----

func cmdExport(reader *bufio.Reader, state *appState) {
	if !state.scanned {
		fmt.Println("  No scan results yet. Run 'scan' first.")
		return
	}

	outputPath := promptRequired(reader, "  Output file path: ")

	fmt.Println("  Filter:")
	fmt.Println("  [1] All results")
	fmt.Println("  [2] 2xx only (falls back to unknown status if no 2xx available)")
	filterStr := prompt(reader, "  Choice [1]: ")
	onlyTwoXX := filterStr == "2"

	f, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("  Error creating output file: %v\n", err)
		return
	}
	defer f.Close()

	written := 0
	for _, ep := range state.results {
		exampleURL := pickExampleURL(ep.Observations, onlyTwoXX)
		if exampleURL == "" {
			continue
		}
		fmt.Fprintln(f, exampleURL)
		written++
	}

	fmt.Printf("  Exported %d URLs to %s\n", written, outputPath)
}

// pickExampleURL selects one representative URL from a list of observations.
// When onlyTwoXX is true it prefers 2xx status codes, falling back to status 0
// (unknown). Entries with a known non-2xx status are skipped when onlyTwoXX is true.
func pickExampleURL(observations []domain.ExampleURL, onlyTwoXX bool) string {
	if !onlyTwoXX {
		if len(observations) == 0 {
			return ""
		}
		return observations[0].URL
	}

	// Try to find a 2xx observation first.
	for _, obs := range observations {
		if obs.StatusCode >= 200 && obs.StatusCode < 300 {
			return obs.URL
		}
	}

	// Fall back to an observation with unknown status (0).
	for _, obs := range observations {
		if obs.StatusCode == 0 {
			return obs.URL
		}
	}

	// All observations have a known non-2xx status — skip this endpoint.
	return ""
}

// ---- command: status (show configured sources) ----

func cmdStatus(state *appState) {
	if len(state.waybackSources) == 0 && len(state.logSources) == 0 && len(state.swaggerSources) == 0 {
		fmt.Println("  No sources configured yet.")
		return
	}

	if len(state.waybackSources) > 0 {
		fmt.Println("  Wayback sources:")
		for _, s := range state.waybackSources {
			fmt.Printf("    domain=%s  prefix=%q\n", s.Domain, s.PathPrefix)
		}
	}
	if len(state.logSources) > 0 {
		fmt.Println("  Log sources:")
		for _, s := range state.logSources {
			fmt.Printf("    file=%s  pattern=%q\n", s.FilePath, s.Pattern)
		}
	}
	if len(state.swaggerSources) > 0 {
		fmt.Println("  Swagger sources:")
		for _, f := range state.swaggerSources {
			fmt.Printf("    file=%s\n", f)
		}
	}
	if state.scanned {
		fmt.Printf("  Last scan: %d endpoint templates.\n", len(state.results))
	}
}

// ---- help text ----

func printHelp() {
	fmt.Println(`
  Commands:
    wayback   Add a Wayback Machine source (domain + optional path prefix)
    logs      Add an access log source (file path + format pattern)
    swagger   Add a Swagger/OpenAPI spec source (file path)
    status    Show configured sources
    scan      Fetch and merge all configured sources
    analyze   Show statistics about scan results
    search    Search endpoints by path prefix and view details
    export    Export a list of example URLs to a file
    help      Show this help text
    quit      Exit the program`)
}
