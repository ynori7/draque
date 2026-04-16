package domain

import (
	"testing"
)

// ---- templateMatchKey unit tests ----

func TestTemplateMatchKey(t *testing.T) {
	cases := []struct {
		name     string
		template string
		want     string
	}{
		{"no params", "/api/users", "/api/users"},
		{"single inferred param", "/api/users/{id}", "/api/users/{}"},
		{"single swagger param", "/api/users/{userId}", "/api/users/{}"},
		{"uuid param", "/api/items/{uuid}", "/api/items/{}"},
		{"multiple params", "/api/{tenant}/users/{id}", "/api/{}/users/{}"},
		{"root only", "/", "/"},
		{"nested multi-param", "/api/v1/users/{userId}/orders/{orderId}", "/api/v1/users/{}/orders/{}"},
		{"already normalized", "/api/users/{}", "/api/users/{}"},
		{"version segment excluded", "/api/v2/users/{id}", "/api/v2/users/{}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := templateMatchKey(tc.template)
			if got != tc.want {
				t.Errorf("templateMatchKey(%q) = %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}

// ---- MatchTemplates tests ----

func TestMatchTemplates_Empty(t *testing.T) {
	result := MatchTemplates()
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}

	result = MatchTemplates([]EndpointTemplate{})
	if len(result) != 0 {
		t.Errorf("expected 0 results for empty source, got %d", len(result))
	}
}

func TestMatchTemplates_SingleSource_NoMerge(t *testing.T) {
	source := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/api/users/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Count:        3,
		},
		{
			Method:       "GET",
			PathTemplate: "/api/items/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Count:        2,
		},
	}

	result := MatchTemplates(source)
	if len(result) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(result))
	}
	if result[0].PathTemplate != "/api/users/{id}" {
		t.Errorf("expected first endpoint /api/users/{id}, got %q", result[0].PathTemplate)
	}
	if result[0].Count != 3 {
		t.Errorf("expected count 3, got %d", result[0].Count)
	}
}

func TestMatchTemplates_BasicMatch(t *testing.T) {
	inferred := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Count:        5,
	}}
	swagger := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{userId}",
		Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
		Count:        0,
	}}

	result := MatchTemplates(inferred, swagger)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged endpoint, got %d", len(result))
	}
	if result[0].Count != 5 {
		t.Errorf("expected count 5, got %d", result[0].Count)
	}
	if result[0].Parameters[0].Name != "userId" {
		t.Errorf("expected swagger param name 'userId', got %q", result[0].Parameters[0].Name)
	}
}

func TestMatchTemplates_NoFalseMatch_DifferentStaticSegments(t *testing.T) {
	source1 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Count:        1,
	}}
	source2 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/items/{id}",
		Count:        1,
	}}

	result := MatchTemplates(source1, source2)
	if len(result) != 2 {
		t.Errorf("expected 2 separate endpoints, got %d", len(result))
	}
}

func TestMatchTemplates_NoFalseMatch_DifferentSegmentCount(t *testing.T) {
	source1 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Count:        1,
	}}
	source2 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}/orders",
		Count:        1,
	}}

	result := MatchTemplates(source1, source2)
	if len(result) != 2 {
		t.Errorf("expected 2 separate endpoints, got %d", len(result))
	}
}

func TestMatchTemplates_SwaggerPreference(t *testing.T) {
	// inferred arrives first — swagger should still win for PathTemplate and Parameters
	inferred := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Count:        5,
	}}
	swagger := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{userId}",
		Parameters:   []Parameter{{Name: "userId", Type: "integer", Source: "swagger"}},
		Count:        0,
	}}

	result := MatchTemplates(inferred, swagger)
	if len(result) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result))
	}
	if result[0].PathTemplate != "/api/users/{userId}" {
		t.Errorf("expected swagger PathTemplate /api/users/{userId}, got %q", result[0].PathTemplate)
	}
	if len(result[0].Parameters) != 1 || result[0].Parameters[0].Name != "userId" {
		t.Errorf("expected swagger param name 'userId', got %+v", result[0].Parameters)
	}
	if result[0].Parameters[0].Source != "swagger" {
		t.Errorf("expected param source 'swagger', got %q", result[0].Parameters[0].Source)
	}
}

func TestMatchTemplates_SwaggerPreference_SwaggerFirst(t *testing.T) {
	// swagger arrives first — should still win
	swagger := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{userId}",
		Parameters:   []Parameter{{Name: "userId", Type: "integer", Source: "swagger"}},
		Count:        0,
	}}
	inferred := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Count:        7,
	}}

	result := MatchTemplates(swagger, inferred)
	if len(result) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result))
	}
	if result[0].PathTemplate != "/api/users/{userId}" {
		t.Errorf("expected swagger PathTemplate, got %q", result[0].PathTemplate)
	}
	if result[0].Parameters[0].Name != "userId" {
		t.Errorf("expected swagger param name 'userId', got %q", result[0].Parameters[0].Name)
	}
	if result[0].Count != 7 {
		t.Errorf("expected count 7, got %d", result[0].Count)
	}
}

func TestMatchTemplates_SumsCounts(t *testing.T) {
	swagger := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/orders/{orderId}",
		Parameters:   []Parameter{{Name: "orderId", Type: "unknown", Source: "swagger"}},
		Count:        0, // swagger never counts
	}}
	wayback := []EndpointTemplate{{
		PathTemplate: "/api/orders/{id}", // empty method → GET
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Count:        4,
	}}
	logs := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/orders/{id}",
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Count:        6,
	}}

	result := MatchTemplates(swagger, wayback, logs)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged endpoint, got %d", len(result))
	}
	if result[0].Count != 10 {
		t.Errorf("expected count 10 (0+4+6), got %d", result[0].Count)
	}
}

func TestMatchTemplates_CombinesObservations(t *testing.T) {
	obs1 := ExampleURL{Source: "wayback", URL: "http://example.com/api/users/123", StatusCode: 200}
	obs2 := ExampleURL{Source: "logs", URL: "http://example.com/api/users/456", StatusCode: 200}
	duplicate := ExampleURL{Source: "logs", URL: "http://example.com/api/users/456", StatusCode: 200} // same as obs2

	source1 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Observations: []ExampleURL{obs1, obs2},
		Count:        2,
	}}
	source2 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{userId}",
		Observations: []ExampleURL{duplicate},
		Count:        1,
	}}

	result := MatchTemplates(source1, source2)
	if len(result) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result))
	}
	if len(result[0].Observations) != 2 {
		t.Errorf("expected 2 unique observations (duplicate removed), got %d: %+v", len(result[0].Observations), result[0].Observations)
	}
	if result[0].Count != 3 {
		t.Errorf("expected count 3, got %d", result[0].Count)
	}
}

func TestMatchTemplates_CombinesExamples(t *testing.T) {
	ex1 := ExampleParameter{ParamName: "id", Value: "123", Source: "wayback"}
	ex2 := ExampleParameter{ParamName: "id", Value: "456", Source: "logs"}
	duplicate := ExampleParameter{ParamName: "id", Value: "456", Source: "logs"} // same as ex2

	source1 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Examples:     []ExampleParameter{ex1, ex2},
		Count:        2,
	}}
	source2 := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{userId}",
		Examples:     []ExampleParameter{duplicate},
		Count:        1,
	}}

	result := MatchTemplates(source1, source2)
	if len(result) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result))
	}
	if len(result[0].Examples) != 2 {
		t.Errorf("expected 2 unique examples (duplicate removed), got %d: %+v", len(result[0].Examples), result[0].Examples)
	}
}

func TestMatchTemplates_ThreeWayMerge(t *testing.T) {
	swagger := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/products/{productId}",
		Parameters:   []Parameter{{Name: "productId", Type: "unknown", Source: "swagger"}},
		Count:        0,
	}}
	// Wayback has empty method — should become GET and match
	wayback := []EndpointTemplate{{
		Method:       "",
		PathTemplate: "/api/products/{id}",
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Observations: []ExampleURL{{Source: "wayback", URL: "http://example.com/api/products/42", StatusCode: 200}},
		Count:        3,
	}}
	logs := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/products/{id}",
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Observations: []ExampleURL{{Source: "logs", URL: "http://example.com/api/products/99", StatusCode: 200}},
		Count:        7,
	}}

	result := MatchTemplates(swagger, wayback, logs)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged endpoint, got %d", len(result))
	}
	ep := result[0]
	if ep.Method != "GET" {
		t.Errorf("expected method GET, got %q", ep.Method)
	}
	if ep.PathTemplate != "/api/products/{productId}" {
		t.Errorf("expected swagger PathTemplate, got %q", ep.PathTemplate)
	}
	if ep.Parameters[0].Name != "productId" || ep.Parameters[0].Source != "swagger" {
		t.Errorf("expected swagger param, got %+v", ep.Parameters)
	}
	if ep.Count != 10 {
		t.Errorf("expected count 10 (0+3+7), got %d", ep.Count)
	}
	if len(ep.Observations) != 2 {
		t.Errorf("expected 2 observations (wayback + logs), got %d", len(ep.Observations))
	}
}

func TestMatchTemplates_MethodAwareGrouping(t *testing.T) {
	// GET and POST should stay separate even with same path shape
	get := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}",
		Count:        2,
	}}
	post := []EndpointTemplate{{
		Method:       "POST",
		PathTemplate: "/api/users/{id}",
		Count:        1,
	}}

	result := MatchTemplates(get, post)
	if len(result) != 2 {
		t.Fatalf("expected 2 endpoints (GET and POST), got %d", len(result))
	}
	methods := map[string]bool{}
	for _, ep := range result {
		methods[ep.Method] = true
	}
	if !methods["GET"] || !methods["POST"] {
		t.Errorf("expected both GET and POST methods, got %v", methods)
	}
}

func TestMatchTemplates_EmptyMethodDefaultsToGET(t *testing.T) {
	// empty-method template (typical of wayback) should merge with explicit GET
	get := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{userId}",
		Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
		Count:        0,
	}}
	noMethod := []EndpointTemplate{{
		Method:       "",
		PathTemplate: "/api/users/{id}",
		Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
		Observations: []ExampleURL{{Source: "wayback", URL: "http://example.com/api/users/7", StatusCode: 200}},
		Count:        4,
	}}

	result := MatchTemplates(get, noMethod)
	if len(result) != 1 {
		t.Fatalf("expected empty method to merge with GET, got %d endpoints", len(result))
	}
	if result[0].Method != "GET" {
		t.Errorf("expected normalized method GET, got %q", result[0].Method)
	}
	if result[0].Count != 4 {
		t.Errorf("expected count 4, got %d", result[0].Count)
	}
	// swagger params should win even when swagger arrived first
	if result[0].Parameters[0].Name != "userId" {
		t.Errorf("expected swagger param name userId, got %q", result[0].Parameters[0].Name)
	}
}

func TestMatchTemplates_PreservesInsertionOrder(t *testing.T) {
	source := []EndpointTemplate{
		{Method: "GET", PathTemplate: "/api/a/{id}", Count: 1},
		{Method: "GET", PathTemplate: "/api/b/{id}", Count: 1},
		{Method: "GET", PathTemplate: "/api/c/{id}", Count: 1},
	}

	result := MatchTemplates(source)
	if len(result) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(result))
	}
	want := []string{"/api/a/{id}", "/api/b/{id}", "/api/c/{id}"}
	for i, ep := range result {
		if ep.PathTemplate != want[i] {
			t.Errorf("result[%d].PathTemplate = %q, want %q", i, ep.PathTemplate, want[i])
		}
	}
}

func TestMatchTemplates_MultipleParams(t *testing.T) {
	swagger := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{userId}/orders/{orderId}",
		Parameters: []Parameter{
			{Name: "userId", Type: "int", Source: "swagger"},
			{Name: "orderId", Type: "uuid", Source: "swagger"},
		},
		Count: 0,
	}}
	inferred := []EndpointTemplate{{
		Method:       "GET",
		PathTemplate: "/api/users/{id}/orders/{uuid}",
		Parameters: []Parameter{
			{Name: "id", Type: "int", Source: "inferred"},
			{Name: "uuid", Type: "uuid", Source: "inferred"},
		},
		Count: 5,
	}}

	result := MatchTemplates(swagger, inferred)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged endpoint, got %d", len(result))
	}
	if result[0].PathTemplate != "/api/users/{userId}/orders/{orderId}" {
		t.Errorf("expected swagger PathTemplate, got %q", result[0].PathTemplate)
	}
	if len(result[0].Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(result[0].Parameters))
	}
	if result[0].Parameters[0].Name != "userId" || result[0].Parameters[1].Name != "orderId" {
		t.Errorf("expected swagger param names, got %+v", result[0].Parameters)
	}
}
