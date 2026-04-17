package domain

import (
	"net/url"
	"regexp"
	"strings"
)

// genericParamRegexp matches the auto-generated parameter names produced by the URL
// normalizer: id, id2, id3, ..., uuid, uuid2, uuid3, ...
var genericParamRegexp = regexp.MustCompile(`^(id\d*|uuid\d*)$`)

// InferIDs is a post-processing step that enriches endpoint templates which have no real
// observations with parameter values inferred from other endpoints in the dataset.
//
// Part 1 — parent/child inference: if an endpoint's path template is a structural prefix of
// another template and each parameter placeholder at the same position has the same name,
// the parent's observed parameter values are inferred for the child endpoint.
//
// Part 2 — shared parameter names: if an endpoint uses a non-generic parameter name
// (anything other than id, id2, uuid, uuid2, etc.) and another endpoint with real
// observations uses the same parameter name, those values are inferred.
//
// Only endpoints with no real observations (from wayback or logs sources) are eligible.
// Inferred examples carry Source "inferred" and are deduplicated.
func InferIDs(endpoints []EndpointTemplate) []EndpointTemplate {
	result := make([]EndpointTemplate, len(endpoints))
	copy(result, endpoints)

	inferFromParentEndpoints(result)
	inferFromMatchingParamNames(result)
	inferObservations(result)

	return result
}

// hasRealValues reports whether an endpoint has at least one observation from a real
// (non-swagger) source (wayback or logs).
func hasRealValues(et EndpointTemplate) bool {
	for _, obs := range et.Observations {
		if obs.Source == "wayback" || obs.Source == "logs" {
			return true
		}
	}
	return false
}

// isGenericParamName reports whether name is one of the auto-generated placeholder names
// produced by the URL normalizer: id, id2, id3, ..., uuid, uuid2, uuid3, ...
func isGenericParamName(name string) bool {
	return genericParamRegexp.MatchString(name)
}

// extractValuesFromObservations extracts the concrete values that appear at parameter
// placeholder positions in template, using the path of each non-swagger observation URL.
// Returns a map of parameter name to a deduplicated list of observed values.
func extractValuesFromObservations(template string, observations []ExampleURL) map[string][]string {
	templateSegs := strings.Split(template, "/")
	result := make(map[string][]string)

	type seenKey struct{ name, value string }
	seen := make(map[seenKey]struct{})

	for _, obs := range observations {
		if obs.Source == "swagger" {
			continue
		}
		parsed, err := url.Parse(obs.URL)
		if err != nil {
			continue
		}
		pathSegs := strings.Split(parsed.Path, "/")
		if len(pathSegs) != len(templateSegs) {
			continue
		}
		for i, seg := range templateSegs {
			if len(seg) < 2 || seg[0] != '{' || seg[len(seg)-1] != '}' {
				continue
			}
			name := seg[1 : len(seg)-1]
			value := pathSegs[i]
			if value == "" {
				continue
			}
			k := seenKey{name, value}
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				result[name] = append(result[name], value)
			}
		}
	}
	return result
}

// inferFromParentEndpoints implements Part 1 of ID inference.
//
// For each endpoint without real observations, find the most structurally specific other
// endpoint that is a strict path-template prefix of it and has real observations, then
// adopt its observed parameter values. Prefix matching requires that at each position:
//   - both segments are static and equal, or
//   - both segments are parameter placeholders with the same name.
//
// If multiple parents qualify, the one with the most segments wins.
func inferFromParentEndpoints(endpoints []EndpointTemplate) {
	for i := range endpoints {
		child := &endpoints[i]
		if hasRealValues(*child) || len(child.Parameters) == 0 {
			continue
		}

		childSegs := strings.Split(child.PathTemplate, "/")

		var bestParent *EndpointTemplate
		bestLen := 0

		for j := range endpoints {
			if i == j {
				continue
			}
			parent := &endpoints[j]
			if !hasRealValues(*parent) {
				continue
			}
			parentSegs := strings.Split(parent.PathTemplate, "/")
			if len(parentSegs) >= len(childSegs) {
				continue // parent must be strictly shorter
			}

			matched := true
			for k, pseg := range parentSegs {
				cseg := childSegs[k]
				pIsParam := len(pseg) >= 2 && pseg[0] == '{' && pseg[len(pseg)-1] == '}'
				cIsParam := len(cseg) >= 2 && cseg[0] == '{' && cseg[len(cseg)-1] == '}'

				switch {
				case pIsParam && cIsParam:
					if pseg[1:len(pseg)-1] != cseg[1:len(cseg)-1] {
						matched = false
					}
				case !pIsParam && !cIsParam:
					if pseg != cseg {
						matched = false
					}
				default:
					matched = false // one is param, other is static
				}
				if !matched {
					break
				}
			}

			if matched && len(parentSegs) > bestLen {
				bestLen = len(parentSegs)
				bestParent = parent
			}
		}

		if bestParent == nil {
			continue
		}

		vals := extractValuesFromObservations(bestParent.PathTemplate, bestParent.Observations)
		for paramName, values := range vals {
			for _, v := range values {
				child.Examples = appendUniqueExamples(child.Examples, ExampleParameter{
					ParamName: paramName,
					Value:     v,
					Source:    "inferred",
				})
			}
		}
	}
}

// inferObservations is the third step of inference. For each endpoint that still has no
// real observations (e.g. swagger-only endpoints), it constructs a synthetic observation
// URL by prepending a host discovered from other endpoints' real observations, substituting
// any available example parameter values into the path template. The result is appended to
// Observations with Source "inferred" so that the rest of the pipeline can treat it like
// any other concrete URL.
func inferObservations(endpoints []EndpointTemplate) {
	// Find the scheme+host from any real (non-swagger) observation in the dataset.
	host := ""
	for _, ep := range endpoints {
		for _, obs := range ep.Observations {
			if obs.Source == "swagger" {
				continue
			}
			if u, err := url.Parse(obs.URL); err == nil && u.Host != "" {
				host = u.Scheme + "://" + u.Host
				break
			}
		}
		if host != "" {
			break
		}
	}
	if host == "" {
		return
	}

	for i := range endpoints {
		ep := &endpoints[i]
		if hasRealValues(*ep) { // no need to infer if we already have real observations
			continue
		}
		if len(ep.Examples) == 0 { // we have no parameter values to substitute, so we can't infer a meaningful URL
			continue
		}

		// Substitute example param values into the path template.
		path := ep.PathTemplate
		for _, ex := range ep.Examples {
			path = strings.ReplaceAll(path, "{"+ex.ParamName+"}", ex.Value)
		}

		ep.Observations = append(ep.Observations, ExampleURL{
			Source: "inferred",
			URL:    host + path,
		})
	}
}

// inferFromMatchingParamNames implements Part 2 of ID inference.
//
// Builds a corpus of non-generic parameter name→value mappings from all endpoints with
// real observations, then applies those values to endpoints with no real observations that
// use the same parameter names. Generic names (id, id2, uuid, uuid2, etc.) are excluded
// because they carry no semantic identity across unrelated endpoints.
func inferFromMatchingParamNames(endpoints []EndpointTemplate) {
	// Build global corpus: non-generic param name → unique observed values.
	type pkey struct{ name, value string }
	seenGlobal := make(map[pkey]struct{})
	globalVals := make(map[string][]string)

	for _, et := range endpoints {
		if !hasRealValues(et) {
			continue
		}
		vals := extractValuesFromObservations(et.PathTemplate, et.Observations)
		for name, values := range vals {
			if isGenericParamName(name) {
				continue
			}
			for _, v := range values {
				k := pkey{name, v}
				if _, ok := seenGlobal[k]; !ok {
					seenGlobal[k] = struct{}{}
					globalVals[name] = append(globalVals[name], v)
				}
			}
		}
	}

	if len(globalVals) == 0 {
		return
	}

	for i := range endpoints {
		ep := &endpoints[i]
		if hasRealValues(*ep) {
			continue
		}
		for _, param := range ep.Parameters {
			if isGenericParamName(param.Name) {
				continue
			}
			for _, v := range globalVals[param.Name] {
				ep.Examples = appendUniqueExamples(ep.Examples, ExampleParameter{
					ParamName: param.Name,
					Value:     v,
					Source:    "inferred",
				})
			}
		}
	}
}
