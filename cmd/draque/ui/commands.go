package ui

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ynori7/draque/domain"
)

// renderHelp returns the help text. When scanned is true, post-scan commands
// (analyze, search, export, reset) are included with fuller descriptions.
func renderHelp(scanned bool) string {
	type entry struct{ cmd, desc string }
	entries := []entry{
		{"wayback", "Add a Wayback Machine source (domain + optional path prefix)"},
		{"logs", "Add an access log source (file or directory path + format pattern)"},
		{"swagger", "Add a Swagger/OpenAPI spec source (file or directory path)"},
		{"status", "Show configured sources and scan summary"},
		{"scan", "Fetch and merge all configured sources (with progress)"},
	}
	if scanned {
		entries = append(entries,
			entry{"analyze", "Show statistics about scan results"},
			entry{"search", "Search endpoints live as you type and view details"},
			entry{"export", "Export one representative URL per endpoint template to a file (one per line);\n             filters: all results or 2xx-status responses only"},
			entry{"reset", "Clear all sources and scan data for a fresh start"},
		)
	}
	entries = append(entries,
		entry{"help", "Show this help text"},
		entry{"quit", "Exit the program"},
	)
	var sb strings.Builder
	sb.WriteString("\n  " + headerStyle.Render("Commands:") + "\n\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("    %-10s  %s\n",
			cmdNameStyle.Render(e.cmd),
			cmdDescStyle.Render(e.desc),
		))
	}
	return sb.String()
}

func renderStatus(state *appState) string {
	if len(state.waybackSources) == 0 && len(state.logSources) == 0 && len(state.swaggerSources) == 0 {
		return "\n  " + subtleStyle.Render("No sources configured yet.") + "\n"
	}
	var sb strings.Builder
	sb.WriteString("\n")
	if len(state.waybackSources) > 0 {
		sb.WriteString("  " + waybackStyle.Render("Wayback sources:") + "\n")
		for _, s := range state.waybackSources {
			sb.WriteString(fmt.Sprintf("    domain=%s  prefix=%s\n",
				waybackStyle.Render(s.Domain),
				subtleStyle.Render(fmt.Sprintf("%q", s.PathPrefix)),
			))
		}
		sb.WriteString("\n")
	}
	if len(state.logSources) > 0 {
		sb.WriteString("  " + logsStyle.Render("Log sources:") + "\n")
		for _, s := range state.logSources {
			sb.WriteString(fmt.Sprintf("    file=%s  pattern=%s\n",
				logsStyle.Render(s.FilePath),
				subtleStyle.Render(fmt.Sprintf("%q", s.Pattern)),
			))
		}
		sb.WriteString("\n")
	}
	if len(state.swaggerSources) > 0 {
		sb.WriteString("  " + swaggerStyle.Render("Swagger sources:") + "\n")
		for _, f := range state.swaggerSources {
			sb.WriteString(fmt.Sprintf("    file=%s\n", swaggerStyle.Render(f)))
		}
		sb.WriteString("\n")
	}
	if state.scanned {
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			subtleStyle.Render("Last scan:"),
			countStyle.Render(fmt.Sprintf("%d endpoint templates", len(state.results))),
		))
	}
	return sb.String()
}

func renderAnalyze(state *appState) string {
	totalCount := 0
	sourceCounts := map[string]int{}
	for _, ep := range state.results {
		totalCount += ep.Count
		for _, obs := range ep.Observations {
			sourceCounts[obs.Source]++
		}
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %-32s %s\n",
		headerStyle.Render("Total unique endpoint templates"),
		countStyle.Render(fmt.Sprintf("%d", len(state.results))),
	))
	sb.WriteString(fmt.Sprintf("  %-32s %s\n",
		headerStyle.Render("Total observations"),
		countStyle.Render(fmt.Sprintf("%d", totalCount)),
	))
	if len(sourceCounts) > 0 {
		sb.WriteString("\n  " + headerStyle.Render("Observations by source:") + "\n")
		for _, src := range []string{"wayback", "logs", "swagger"} {
			if n, ok := sourceCounts[src]; ok {
				sb.WriteString(fmt.Sprintf("    %s  %s\n",
					sourceStyle(src).Render(fmt.Sprintf("%-10s", src)),
					countStyle.Render(fmt.Sprintf("%d", n)),
				))
			}
		}
	}
	sorted := make([]domain.EndpointTemplate, len(state.results))
	copy(sorted, state.results)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })
	top := sorted
	if len(top) > 10 {
		top = top[:10]
	}
	sb.WriteString("\n  " + headerStyle.Render("Top endpoints by observation count:") + "\n")
	for i, ep := range top {
		sb.WriteString(fmt.Sprintf("    %s  %s  %-50s  %s\n",
			subtleStyle.Render(fmt.Sprintf("%2d.", i+1)),
			methodColor(ep.Method).Render(fmt.Sprintf("%-6s", ep.Method)),
			pathStyle.Render(ep.PathTemplate),
			countStyle.Render(fmt.Sprintf("count: %d", ep.Count)),
		))
	}

	// Aggregate route parameters across all endpoints.
	// For each param name, count how many unique route templates contain it.
	paramRouteCounts := buildParamRouteCounts(state.results)
	if len(paramRouteCounts) > 0 {
		type paramCount struct {
			name  string
			count int
		}
		params := make([]paramCount, 0, len(paramRouteCounts))
		for name, count := range paramRouteCounts {
			params = append(params, paramCount{name: name, count: count})
		}
		sort.Slice(params, func(i, j int) bool {
			if params[i].count != params[j].count {
				return params[i].count > params[j].count
			}
			return params[i].name < params[j].name
		})
		topParams := params
		if len(topParams) > 10 {
			topParams = topParams[:10]
		}
		sb.WriteString("\n  " + headerStyle.Render("Top route parameters") + " " +
			subtleStyle.Render("(count = unique routes where parameter appears):") + "\n")
		for i, pc := range topParams {
			sb.WriteString(fmt.Sprintf("    %s  %-30s  %s\n",
				subtleStyle.Render(fmt.Sprintf("%2d.", i+1)),
				paramStyle.Render(fmt.Sprintf("{%s}", pc.name)),
				countStyle.Render(fmt.Sprintf("routes: %d", pc.count)),
			))
		}
	}

	return sb.String()
}

func renderEndpointDetail(ep domain.EndpointTemplate) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  %s %s %s\n",
		subtleStyle.Render("----"),
		methodColor(ep.Method).Render(ep.Method)+" "+headerStyle.Render(ep.PathTemplate),
		subtleStyle.Render("----"),
	))
	sb.WriteString(fmt.Sprintf("  %s  %s\n",
		headerStyle.Render("Count:"),
		countStyle.Render(fmt.Sprintf("%d", ep.Count)),
	))
	sourceSet := map[string]struct{}{}
	for _, obs := range ep.Observations {
		sourceSet[obs.Source] = struct{}{}
	}
	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	coloredSources := make([]string, 0, len(sources))
	for _, s := range sources {
		coloredSources = append(coloredSources, sourceStyle(s).Render(s))
	}
	sb.WriteString(fmt.Sprintf("  %s  %s\n",
		headerStyle.Render("Sources:"),
		strings.Join(coloredSources, subtleStyle.Render(", ")),
	))
	if len(ep.Parameters) > 0 {
		sb.WriteString("\n  " + headerStyle.Render("Parameters:") + "\n")
		for _, p := range ep.Parameters {
			sb.WriteString(fmt.Sprintf("    %s  type: %-10s  source: %s\n",
				paramStyle.Render(fmt.Sprintf("{%s}", p.Name)),
				p.Type,
				sourceStyle(p.Source).Render(p.Source),
			))
		}
	}
	if len(ep.Examples) > 0 {
		sb.WriteString("\n  " + headerStyle.Render("Observed parameter values:") + "\n")
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
				strs = append(strs, fmt.Sprintf("%s %s",
					v.Value, subtleStyle.Render(fmt.Sprintf("(from %s)", v.Source))))
			}
			sb.WriteString(fmt.Sprintf("    %s: %s\n",
				paramStyle.Render(fmt.Sprintf("{%s}", name)),
				strings.Join(strs, subtleStyle.Render(", ")),
			))
		}
	}
	if len(ep.Observations) > 0 {
		sb.WriteString("\n  " + headerStyle.Render("Observed URLs:") + "\n")
		for _, obs := range ep.Observations {
			statusStr := ""
			if obs.StatusCode != 0 {
				statusStr = statusCodeStyle(obs.StatusCode).Render(fmt.Sprintf(" [%d]", obs.StatusCode))
			}
			sb.WriteString(fmt.Sprintf("    %s%s %s\n",
				sourceStyle(obs.Source).Render(fmt.Sprintf("[%-7s]", obs.Source)),
				statusStr,
				subtleStyle.Render(obs.URL),
			))
		}
	}
	return sb.String()
}

// buildParamRouteCounts returns a map from route parameter name to the number of
// unique route templates (METHOD + path) in which that parameter appears.
func buildParamRouteCounts(results []domain.EndpointTemplate) map[string]int {
	paramRoutes := make(map[string]map[string]struct{})
	for _, ep := range results {
		for _, p := range ep.Parameters {
			if paramRoutes[p.Name] == nil {
				paramRoutes[p.Name] = make(map[string]struct{})
			}
			paramRoutes[p.Name][ep.Method+"\x00"+ep.PathTemplate] = struct{}{}
		}
	}
	if len(paramRoutes) == 0 {
		return nil
	}
	counts := make(map[string]int, len(paramRoutes))
	for name, routes := range paramRoutes {
		counts[name] = len(routes)
	}
	return counts
}

// renderParameterDetail renders the detail view for a selected route parameter,
// showing its inferred type, sources, example values, and routes where it appears.
func renderParameterDetail(name string, results []domain.EndpointTemplate) string {
	type routeInfo struct {
		method   string
		template string
	}
	var routes []routeInfo
	typeSet := make(map[string]struct{})
	sourceSet := make(map[string]struct{})
	var examples []domain.ExampleParameter

	for _, ep := range results {
		found := false
		for _, p := range ep.Parameters {
			if p.Name == name {
				found = true
				typeSet[p.Type] = struct{}{}
				sourceSet[p.Source] = struct{}{}
				break
			}
		}
		if !found {
			continue
		}
		routes = append(routes, routeInfo{ep.Method, ep.PathTemplate})
		for _, ex := range ep.Examples {
			if ex.ParamName == name {
				examples = append(examples, ex)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  %s %s %s\n",
		subtleStyle.Render("----"),
		headerStyle.Render("route param: ")+paramStyle.Render(fmt.Sprintf("{%s}", name)),
		subtleStyle.Render("----"),
	))
	sb.WriteString(fmt.Sprintf("  %s  %s\n",
		headerStyle.Render("Unique routes:"),
		countStyle.Render(fmt.Sprintf("%d", len(routes))),
	))

	if len(typeSet) > 0 {
		types := make([]string, 0, len(typeSet))
		for t := range typeSet {
			types = append(types, t)
		}
		sort.Strings(types)
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			headerStyle.Render("Types:"),
			strings.Join(types, ", "),
		))
	}

	if len(sourceSet) > 0 {
		srcs := make([]string, 0, len(sourceSet))
		for s := range sourceSet {
			srcs = append(srcs, s)
		}
		sort.Strings(srcs)
		colored := make([]string, 0, len(srcs))
		for _, s := range srcs {
			colored = append(colored, sourceStyle(s).Render(s))
		}
		sb.WriteString(fmt.Sprintf("  %s  %s\n",
			headerStyle.Render("Sources:"),
			strings.Join(colored, subtleStyle.Render(", ")),
		))
	}

	if len(examples) > 0 {
		sb.WriteString("\n  " + headerStyle.Render("Example values:") + "\n")
		shown := examples
		if len(shown) > 10 {
			shown = shown[:10]
		}
		for _, ex := range shown {
			sb.WriteString(fmt.Sprintf("    %s %s\n",
				ex.Value,
				subtleStyle.Render(fmt.Sprintf("(from %s)", ex.Source)),
			))
		}
		if len(examples) > 10 {
			sb.WriteString(fmt.Sprintf("    %s\n",
				subtleStyle.Render(fmt.Sprintf("  … and %d more", len(examples)-10)),
			))
		}
	}

	if len(routes) > 0 {
		const maxRoutes = 5
		sb.WriteString("\n  " + headerStyle.Render("Routes where seen:") + "\n")
		shown := routes
		if len(shown) > maxRoutes {
			shown = shown[:maxRoutes]
		}
		for _, r := range shown {
			sb.WriteString(fmt.Sprintf("    %s  %s\n",
				methodColor(r.method).Render(fmt.Sprintf("%-6s", r.method)),
				pathStyle.Render(r.template),
			))
		}
		if len(routes) > maxRoutes {
			sb.WriteString(fmt.Sprintf("    %s\n",
				subtleStyle.Render(fmt.Sprintf("  … and %d more", len(routes)-maxRoutes)),
			))
		}
	}

	return sb.String()
}

// pickExampleURL selects one representative URL from a list of observations.// When onlyTwoXX is true it prefers 2xx status codes, falling back to status 0
// (unknown). Entries with a known non-2xx status are skipped when onlyTwoXX is true.
func pickExampleURL(observations []domain.ExampleURL, onlyTwoXX bool) string {
	if !onlyTwoXX {
		if len(observations) == 0 {
			return ""
		}
		return observations[0].URL
	}
	for _, obs := range observations {
		if obs.StatusCode >= 200 && obs.StatusCode < 300 {
			return obs.URL
		}
	}
	for _, obs := range observations {
		if obs.StatusCode == 0 {
			return obs.URL
		}
	}
	return ""
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
		return nil
	}
	return fmt.Errorf("the log format pattern did not match the first %d line(s) of %s", checked, filePath)
}
