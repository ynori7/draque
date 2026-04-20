package domain

import (
	"strings"
)

// MatchTemplates merges endpoint templates from multiple sources into a deduplicated list,
// then runs ID and observation inference on the merged result.
// Templates from different sources match when they have the same HTTP method and their path
// templates share the same static segments (parameter placeholders are treated as wildcards).
//
// An empty Method is treated as GET. When merging, swagger-sourced parameter names and the
// swagger PathTemplate are preferred. Observations and examples are combined across sources
// with duplicates removed. Counts are summed (swagger templates have Count=0 and do not
// inflate the total).
func MatchTemplates(sources ...[]EndpointTemplate) []EndpointTemplate {
	return matchTemplatesInternal(ScanLimits{}, sources...)
}

// MatchTemplatesWithLimits is like MatchTemplates but applies per-endpoint size caps
// during merging and inference as configured in limits.
func MatchTemplatesWithLimits(limits ScanLimits, sources ...[]EndpointTemplate) []EndpointTemplate {
	return matchTemplatesInternal(limits, sources...)
}

func matchTemplatesInternal(limits ScanLimits, sources ...[]EndpointTemplate) []EndpointTemplate {
	byKey := make(map[string]*EndpointTemplate)
	order := make([]string, 0)

	for _, source := range sources {
		for _, et := range source {
			method := strings.ToUpper(strings.TrimSpace(et.Method))
			if method == "" {
				method = "GET"
			}
			et.Method = method

			key := method + "\x00" + templateMatchKey(et.PathTemplate)
			if existing, ok := byKey[key]; ok {
				mergeInto(existing, et, limits)
			} else {
				copy := et
				byKey[key] = &copy
				order = append(order, key)
			}
		}
	}

	result := make([]EndpointTemplate, 0, len(order))
	for _, k := range order {
		result = append(result, *byKey[k])
	}
	return inferIDsInternal(result, limits)
}

// templateMatchKey returns a normalized form of a path template for equality comparison.
// Every parameter placeholder segment ({...}) is replaced with the fixed token {}
// so that templates with different parameter names (e.g. {id} vs {userId}) compare equal.
func templateMatchKey(template string) string {
	segments := strings.Split(template, "/")
	for i, seg := range segments {
		if len(seg) >= 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			segments[i] = "{}"
		}
	}
	return strings.Join(segments, "/")
}

// mergeInto folds other into base in-place using the merge rules from SPEC 6:
//   - PathTemplate and Parameters: prefer the swagger-sourced set
//   - Examples: combine across both, removing exact duplicates
//   - Observations: combine across both, removing exact duplicates
//   - Count: summed (swagger count is 0 and does not inflate the total)
func mergeInto(base *EndpointTemplate, other EndpointTemplate, limits ScanLimits) {
	if !hasSwaggerParams(*base) && hasSwaggerParams(other) {
		base.PathTemplate = other.PathTemplate
		base.Parameters = other.Parameters
	}
	base.Examples = appendUniqueExamples(base.Examples, limits.MaxExamples, other.Examples...)
	base.Observations = appendUniqueObservations(base.Observations, limits.MaxObservations, other.Observations...)
	base.Count += other.Count
}

func hasSwaggerParams(et EndpointTemplate) bool {
	for _, p := range et.Parameters {
		if p.Source == "swagger" {
			return true
		}
	}
	return false
}

func appendUniqueExamples(dst []ExampleParameter, max int, src ...ExampleParameter) []ExampleParameter {
	if max > 0 && len(dst) >= max {
		return dst
	}
	type key struct{ ParamName, Value, Source string }
	seen := make(map[key]struct{}, len(dst))
	for _, e := range dst {
		seen[key{e.ParamName, e.Value, e.Source}] = struct{}{}
	}
	for _, e := range src {
		if max > 0 && len(dst) >= max {
			break
		}
		k := key{e.ParamName, e.Value, e.Source}
		if _, ok := seen[k]; !ok {
			dst = append(dst, e)
			seen[k] = struct{}{}
		}
	}
	return dst
}

func appendUniqueObservations(dst []ExampleURL, max int, src ...ExampleURL) []ExampleURL {
	if max > 0 && len(dst) >= max {
		return dst
	}
	type key struct {
		Source     string
		URL        string
		StatusCode int
	}
	seen := make(map[key]struct{}, len(dst))
	for _, o := range dst {
		seen[key{o.Source, o.URL, o.StatusCode}] = struct{}{}
	}
	for _, o := range src {
		if max > 0 && len(dst) >= max {
			break
		}
		k := key{o.Source, o.URL, o.StatusCode}
		if _, ok := seen[k]; !ok {
			dst = append(dst, o)
			seen[k] = struct{}{}
		}
	}
	return dst
}
