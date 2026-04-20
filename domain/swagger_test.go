package domain

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// sortEndpoints sorts endpoints by method+path for deterministic comparison,
// since map iteration order in extractEndpoints is non-deterministic.
func sortEndpoints(endpoints []EndpointTemplate) {
	sort.Slice(endpoints, func(i, j int) bool {
		ki := endpoints[i].Method + "\x00" + endpoints[i].PathTemplate
		kj := endpoints[j].Method + "\x00" + endpoints[j].PathTemplate
		return ki < kj
	})
}

func TestParseSwaggerSpec(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		file    string
		content string
		want    []EndpointTemplate
		wantErr bool
	}{
		{
			name: "openapi v2 json - simple path no params",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"info": {"title": "Test", "version": "1.0"},
				"paths": {
					"/health": {
						"get": {"responses": {"200": {"description": "ok"}}}
					}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/health"},
			},
		},
		{
			name: "openapi v2 json - path param integer",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/users/{userId}": {
						"get": {
							"parameters": [
								{"name": "userId", "in": "path", "type": "integer"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/users/{userId}",
					Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 json - multiple methods on same path",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/items/{itemId}": {
						"get": {
							"parameters": [
								{"name": "itemId", "in": "path", "type": "integer"}
							]
						},
						"delete": {
							"parameters": [
								{"name": "itemId", "in": "path", "type": "integer"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "DELETE",
					PathTemplate: "/items/{itemId}",
					Parameters:   []Parameter{{Name: "itemId", Type: "int", Source: "swagger"}},
				},
				{
					Method:       "GET",
					PathTemplate: "/items/{itemId}",
					Parameters:   []Parameter{{Name: "itemId", Type: "int", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 json - uuid format param",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/orders/{orderId}": {
						"get": {
							"parameters": [
								{"name": "orderId", "in": "path", "type": "string", "format": "uuid"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/orders/{orderId}",
					Parameters:   []Parameter{{Name: "orderId", Type: "uuid", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 json - string param",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/search/{keyword}": {
						"get": {
							"parameters": [
								{"name": "keyword", "in": "path", "type": "string"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/search/{keyword}",
					Parameters:   []Parameter{{Name: "keyword", Type: "string", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 json - unknown type param",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/data/{ref}": {
						"get": {
							"parameters": [
								{"name": "ref", "in": "path", "type": "object"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/data/{ref}",
					Parameters:   []Parameter{{Name: "ref", Type: "unknown", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 json - path-level parameters",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/users/{userId}/orders/{orderId}": {
						"parameters": [
							{"name": "userId", "in": "path", "type": "integer"}
						],
						"get": {
							"parameters": [
								{"name": "orderId", "in": "path", "type": "string", "format": "uuid"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/users/{userId}/orders/{orderId}",
					Parameters: []Parameter{
						{Name: "userId", Type: "int", Source: "swagger"},
						{Name: "orderId", Type: "uuid", Source: "swagger"},
					},
				},
			},
		},
		{
			name: "openapi v2 json - operation param overrides path-level param",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/items/{id}": {
						"parameters": [
							{"name": "id", "in": "path", "type": "string"}
						],
						"get": {
							"parameters": [
								{"name": "id", "in": "path", "type": "integer"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/items/{id}",
					Parameters:   []Parameter{{Name: "id", Type: "int", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 json - query params ignored",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/users/{userId}": {
						"get": {
							"parameters": [
								{"name": "userId", "in": "path", "type": "integer"},
								{"name": "page", "in": "query", "type": "integer"},
								{"name": "X-Auth", "in": "header", "type": "string"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/users/{userId}",
					Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 json - top-level metadata fields and basePath",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"info": {"title": "Test", "version": "1.0"},
				"host": "example.com",
				"basePath": "/api",
				"schemes": ["https"],
				"paths": {
					"/ping": {
						"get": {
							"tags": ["health"],
							"summary": "Ping",
							"description": "Returns pong",
							"operationId": "ping",
							"security": [{"apiKey": []}],
							"responses": {
								"200": {"description": "pong"}
							},
							"x-custom-extension": "value"
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/api/ping"},
			},
		},
		{
			name: "openapi v3 json - basic",
			file: "spec.json",
			content: `{
				"openapi": "3.0.0",
				"info": {"title": "Test", "version": "1.0"},
				"paths": {
					"/users/{userId}": {
						"get": {
							"parameters": [
								{
									"name": "userId",
									"in": "path",
									"schema": {"type": "integer"}
								}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/users/{userId}",
					Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v3 json - uuid in schema",
			file: "spec.json",
			content: `{
				"openapi": "3.1.0",
				"paths": {
					"/orders/{orderId}": {
						"get": {
							"parameters": [
								{
									"name": "orderId",
									"in": "path",
									"schema": {"type": "string", "format": "uuid"}
								}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/orders/{orderId}",
					Parameters:   []Parameter{{Name: "orderId", Type: "uuid", Source: "swagger"}},
				},
			},
		},
		{
			name: "yaml format",
			file: "spec.yaml",
			content: `swagger: "2.0"
paths:
  /users/{userId}:
    get:
      parameters:
        - name: userId
          in: path
          type: integer
`,
			want: []EndpointTemplate{
				{
					Method:       "GET",
					PathTemplate: "/users/{userId}",
					Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
				},
			},
		},
		{
			name: "yml extension",
			file: "spec.yml",
			content: `swagger: "2.0"
paths:
  /ping:
    get: {}
`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/ping"},
			},
		},
		// --- servers prefix (OpenAPI v3) ---
		{
			name: "openapi v3 json - servers prefix applied to all paths",
			file: "spec.json",
			content: `{
				"openapi": "3.0.0",
				"servers": [{"url": "https://api.example.com/v1/users"}],
				"paths": {
					"/getUser": {
						"get": {}
					},
					"/{userId}": {
						"delete": {
							"parameters": [
								{"name": "userId", "in": "path", "schema": {"type": "integer"}}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{
					Method:       "DELETE",
					PathTemplate: "/v1/users/{userId}",
					Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
				},
				{Method: "GET", PathTemplate: "/v1/users/getUser"},
			},
		},
		{
			name: "openapi v3 yaml - servers prefix applied",
			file: "spec.yaml",
			content: `openapi: "3.0.0"
servers:
  - url: https://api.example.com/api/v2
paths:
  /ping:
    get: {}
`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/api/v2/ping"},
			},
		},
		{
			name: "openapi v3 json - servers url with no path prefix ignored",
			file: "spec.json",
			content: `{
				"openapi": "3.0.0",
				"servers": [{"url": "https://api.example.com"}],
				"paths": {
					"/health": {"get": {}}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/health"},
			},
		},
		{
			name: "openapi v3 json - servers url with root path ignored",
			file: "spec.json",
			content: `{
				"openapi": "3.0.0",
				"servers": [{"url": "https://api.example.com/"}],
				"paths": {
					"/health": {"get": {}}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/health"},
			},
		},
		{
			name: "openapi v3 json - multiple servers uses first entry",
			file: "spec.json",
			content: `{
				"openapi": "3.0.0",
				"servers": [
					{"url": "https://api.example.com/v1"},
					{"url": "https://api.example.com/v2"}
				],
				"paths": {
					"/items": {"get": {}}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/v1/items"},
			},
		},
		{
			name: "openapi v3 json - no servers section no prefix",
			file: "spec.json",
			content: `{
				"openapi": "3.0.0",
				"paths": {
					"/status": {"get": {}}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/status"},
			},
		},
		// --- basePath (OpenAPI v2) ---
		{
			name: "openapi v2 json - basePath applied to all paths",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"basePath": "/api/v1",
				"paths": {
					"/users": {"get": {}},
					"/users/{userId}": {
						"get": {
							"parameters": [
								{"name": "userId", "in": "path", "type": "integer"}
							]
						}
					}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/api/v1/users"},
				{
					Method:       "GET",
					PathTemplate: "/api/v1/users/{userId}",
					Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
				},
			},
		},
		{
			name: "openapi v2 yaml - basePath applied",
			file: "spec.yaml",
			content: `swagger: "2.0"
basePath: /v2
paths:
  /ping:
    get: {}
`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/v2/ping"},
			},
		},
		{
			name: "openapi v2 json - basePath of root slash ignored",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"basePath": "/",
				"paths": {
					"/health": {"get": {}}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/health"},
			},
		},
		{
			name: "openapi v2 json - no basePath no prefix",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"paths": {
					"/status": {"get": {}}
				}
			}`,
			want: []EndpointTemplate{
				{Method: "GET", PathTemplate: "/status"},
			},
		},
		{
			name: "missing paths key returns empty slice",
			file: "spec.json",
			content: `{
				"swagger": "2.0",
				"info": {"title": "empty"}
			}`,
			want: []EndpointTemplate{},
		},
		{
			name:    "malformed json returns error",
			file:    "spec.json",
			content: `{ this is not valid json`,
			wantErr: true,
		},
		{
			name:    "malformed yaml returns error",
			file:    "spec.yaml",
			content: "paths:\n  - [invalid: yaml",
			wantErr: true,
		},
		{
			name:    "unknown swagger version returns error",
			file:    "spec.json",
			content: `{"swagger": "1.0", "paths": {}}`,
			wantErr: true,
		},
		{
			name:    "unknown openapi version returns error",
			file:    "spec.json",
			content: `{"openapi": "4.0.0", "paths": {}}`,
			wantErr: true,
		},
		{
			name:    "missing version field returns error",
			file:    "spec.json",
			content: `{"info": {"title": "no version"}, "paths": {}}`,
			wantErr: true,
		},
		{
			name:    "empty file returns error",
			file:    "spec.json",
			content: `null`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.file)
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			got, err := ParseSwaggerSpec(path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sortEndpoints(got)
			sortEndpoints(tc.want)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("mismatch:\n  got  %+v\n  want %+v", got, tc.want)
			}
		})
	}
}

func TestParseSwaggerSpecFileNotFound(t *testing.T) {
	_, err := ParseSwaggerSpec("/nonexistent/path/spec.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
