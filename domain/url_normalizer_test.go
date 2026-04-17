package domain

import (
	"reflect"
	"testing"
)

func TestNormalizeURLTemplate(t *testing.T) {
	t.Helper()

	tests := []struct {
		name           string
		rawURL         string
		wantTemplate   string
		wantParams     []Parameter
		wantNormalized string
		wantErr        bool
	}{
		{
			name:           "static path unchanged",
			rawURL:         "https://example.com/api/users",
			wantTemplate:   "/api/users",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/api/users",
		},
		{
			name:         "single numeric segment becomes id",
			rawURL:       "https://example.com/api/users/123",
			wantTemplate: "/api/users/{id}",
			wantParams: []Parameter{
				{Name: "id", Type: "int", Source: "inferred"},
			},
			wantNormalized: "https://example.com/api/users/123",
		},
		{
			name:         "mixed alphanumeric segment with digit becomes id",
			rawURL:       "https://example.com/api/items/1a3b",
			wantTemplate: "/api/items/{id}",
			wantParams: []Parameter{
				{Name: "id", Type: "int", Source: "inferred"},
			},
			wantNormalized: "https://example.com/api/items/1a3b",
		},
		{
			name:         "uuid segment becomes uuid",
			rawURL:       "https://example.com/api/orders/550e8400-e29b-41d4-a716-446655440000",
			wantTemplate: "/api/orders/{uuid}",
			wantParams: []Parameter{
				{Name: "uuid", Type: "uuid", Source: "inferred"},
			},
			wantNormalized: "https://example.com/api/orders/550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:         "uuid matching is case-insensitive",
			rawURL:       "https://example.com/api/orders/550E8400-E29B-41D4-A716-446655440000",
			wantTemplate: "/api/orders/{uuid}",
			wantParams: []Parameter{
				{Name: "uuid", Type: "uuid", Source: "inferred"},
			},
			wantNormalized: "https://example.com/api/orders/550E8400-E29B-41D4-A716-446655440000",
		},
		{
			name:         "numeric then uuid gets id and uuid",
			rawURL:       "https://example.com/api/users/123/orders/550e8400-e29b-41d4-a716-446655440000",
			wantTemplate: "/api/users/{id}/orders/{uuid}",
			wantParams: []Parameter{
				{Name: "id", Type: "int", Source: "inferred"},
				{Name: "uuid", Type: "uuid", Source: "inferred"},
			},
			wantNormalized: "https://example.com/api/users/123/orders/550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:         "multiple numeric segments get id then id2",
			rawURL:       "https://example.com/api/users/123/items/456",
			wantTemplate: "/api/users/{id}/items/{id2}",
			wantParams: []Parameter{
				{Name: "id", Type: "int", Source: "inferred"},
				{Name: "id2", Type: "int", Source: "inferred"},
			},
			wantNormalized: "https://example.com/api/users/123/items/456",
		},
		{
			name:         "multiple numeric segments get id, id2, id3",
			rawURL:       "https://example.com/a/1/b/2/c/3",
			wantTemplate: "/a/{id}/b/{id2}/c/{id3}",
			wantParams: []Parameter{
				{Name: "id", Type: "int", Source: "inferred"},
				{Name: "id2", Type: "int", Source: "inferred"},
				{Name: "id3", Type: "int", Source: "inferred"},
			},
			wantNormalized: "https://example.com/a/1/b/2/c/3",
		},
		{
			name:         "multiple uuid segments get uuid then uuid2",
			rawURL:       "https://example.com/a/550e8400-e29b-41d4-a716-446655440000/b/660e8400-e29b-41d4-a716-446655440000",
			wantTemplate: "/a/{uuid}/b/{uuid2}",
			wantParams: []Parameter{
				{Name: "uuid", Type: "uuid", Source: "inferred"},
				{Name: "uuid2", Type: "uuid", Source: "inferred"},
			},
			wantNormalized: "https://example.com/a/550e8400-e29b-41d4-a716-446655440000/b/660e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:           "v1 prefix stays static",
			rawURL:         "https://example.com/v1/users",
			wantTemplate:   "/v1/users",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/v1/users",
		},
		{
			name:           "v2 prefix stays static",
			rawURL:         "https://example.com/v2/users",
			wantTemplate:   "/v2/users",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/v2/users",
		},
		{
			name:           "b2b stays static",
			rawURL:         "https://example.com/api/b2b/orders",
			wantTemplate:   "/api/b2b/orders",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/api/b2b/orders",
		},
		{
			name:           "e2e stays static",
			rawURL:         "https://example.com/api/e2e/tests",
			wantTemplate:   "/api/e2e/tests",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/api/e2e/tests",
		},
		{
			name:           "p2p stays static",
			rawURL:         "https://example.com/api/p2p/nodes",
			wantTemplate:   "/api/p2p/nodes",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/api/p2p/nodes",
		},
		{
			name:           "segment ending with whitelisted suffix stays static",
			rawURL:         "https://example.com/users-v2/me",
			wantTemplate:   "/users-v2/me",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/users-v2/me",
		},
		{
			name:           "segment ending with whitelisted suffix (camelCase) stays static, sibling with digit becomes id",
			rawURL:         "https://example.com/usersV2/1a2Z3",
			wantTemplate:   "/usersV2/{id}",
			wantParams:     []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			wantNormalized: "https://example.com/usersV2/1a2Z3",
		},
		{
			name:           "segment starting with whitelisted prefix stays static",
			rawURL:         "https://example.com/e2e-test/generateUser",
			wantTemplate:   "/e2e-test/generateUser",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/e2e-test/generateUser",
		},
		{
			name:         "query string sorted deterministically",
			rawURL:       "https://example.com/api/users?b=2&a=1",
			wantTemplate: "/api/users",
			wantParams:   []Parameter{},
			// a comes before b after sorting
			wantNormalized: "https://example.com/api/users?a=1&b=2",
		},
		{
			name:           "fragment stripped from normalized URL",
			rawURL:         "https://example.com/api/users?b=2&a=1#section",
			wantTemplate:   "/api/users",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/api/users?a=1&b=2",
		},
		{
			name:           "default https port removed",
			rawURL:         "https://example.com:443/api/users",
			wantTemplate:   "/api/users",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/api/users",
		},
		{
			name:           "default http port removed",
			rawURL:         "http://example.com:80/api/users",
			wantTemplate:   "/api/users",
			wantParams:     []Parameter{},
			wantNormalized: "http://example.com/api/users",
		},
		{
			name:           "non-default port preserved",
			rawURL:         "https://example.com:8443/api/users",
			wantTemplate:   "/api/users",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com:8443/api/users",
		},
		{
			name:           "host lowercased",
			rawURL:         "https://EXAMPLE.COM/api/users",
			wantTemplate:   "/api/users",
			wantParams:     []Parameter{},
			wantNormalized: "https://example.com/api/users",
		},
		{
			name:    "ftp scheme is rejected",
			rawURL:  "ftp://example.com/api/users",
			wantErr: true,
		},
		{
			name:    "relative URL is rejected",
			rawURL:  "/api/users",
			wantErr: true,
		},
		{
			name:    "root path is rejected",
			rawURL:  "https://example.com/",
			wantErr: true,
		},
		{
			name:    "invalid URL is rejected",
			rawURL:  "not a url",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			np, normalized, err := NormalizeURL(tc.rawURL)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none; NormalizedPath=%+v normalized=%q", np, normalized)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if np.Template != tc.wantTemplate {
				t.Errorf("template\n  want: %q\n   got: %q", tc.wantTemplate, np.Template)
			}

			if !reflect.DeepEqual(np.Parameters, tc.wantParams) {
				t.Errorf("parameters\n  want: %#v\n   got: %#v", tc.wantParams, np.Parameters)
			}

			if normalized != tc.wantNormalized {
				t.Errorf("normalized URL\n  want: %q\n   got: %q", tc.wantNormalized, normalized)
			}
		})
	}
}

// TestNormalizeURLQueryDoesNotAffectTemplate demonstrates that different query strings produce
// the same template but different normalized URLs.
func TestNormalizeURLQueryDoesNotAffectTemplate(t *testing.T) {
	t.Helper()

	urls := []string{
		"https://example.com/api/users?a=1",
		"https://example.com/api/users?b=2",
		"https://example.com/api/users?a=1&b=2",
		"https://example.com/api/users",
	}

	for _, rawURL := range urls {
		np, _, err := NormalizeURL(rawURL)
		if err != nil {
			t.Fatalf("NormalizeURL(%q) error: %v", rawURL, err)
		}
		if np.Template != "/api/users" {
			t.Errorf("NormalizeURL(%q): want template /api/users, got %q", rawURL, np.Template)
		}
	}
}

// TestNormalizeURLQueryAffectsNormalizedURL proves that different query content results in different normalized URLs.
func TestNormalizeURLQueryAffectsNormalizedURL(t *testing.T) {
	t.Helper()

	_, url1, err := NormalizeURL("https://example.com/api/users?a=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, url2, err := NormalizeURL("https://example.com/api/users?a=2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url1 == url2 {
		t.Errorf("expected different normalized URLs for different query content, got same: %q", url1)
	}
}
