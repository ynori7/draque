package domain

import (
	"encoding/json"
	"testing"
)

func TestEndpointTemplateJSONSerialization(t *testing.T) {
	tmpl := EndpointTemplate{
		Method:       "GET",
		PathTemplate: "/users/{id}",
		Parameters: []Parameter{
			{Name: "id", Type: "uuid", Source: "swagger"},
		},
		Examples: []ExampleParameter{
			{ParamName: "id", Value: "123e4567-e89b-12d3-a456-426614174000", Source: "logs"},
		},
		Observations: []ExampleURL{
			{Source: "wayback", URL: "https://example.com/users/123e4567-e89b-12d3-a456-426614174000"},
		},
		Count: 1,
	}

	b, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, field := range []string{"method", "path_template", "parameters", "examples", "observations", "count"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("expected JSON field %q to be present", field)
		}
	}
}

func TestConstructProgrammatically(t *testing.T) {
	tmpl := EndpointTemplate{}
	tmpl.Method = "POST"
	tmpl.PathTemplate = "/widgets/{widget_id}"
	tmpl.Parameters = append(tmpl.Parameters, Parameter{Name: "widget_id", Type: "int", Source: "inferred"})
	tmpl.Examples = append(tmpl.Examples, ExampleParameter{ParamName: "widget_id", Value: "42", Source: "wayback"})
	tmpl.Observations = append(tmpl.Observations, ExampleURL{Source: "logs", URL: "https://example.com/widgets/42"})
	tmpl.Count++

	if tmpl.Method != "POST" || tmpl.PathTemplate == "" || tmpl.Count != 1 {
		t.Fatalf("unexpected object state: %+v", tmpl)
	}
}

func TestNormalizedPathJSONSerialization(t *testing.T) {
	np := NormalizedPath{
		Template: "/api/users/{id}",
		Parameters: []Parameter{
			{Name: "id", Type: "int", Source: "inferred"},
		},
	}

	b, err := json.Marshal(np)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, field := range []string{"template", "parameters"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("expected JSON field %q to be present", field)
		}
	}
}
