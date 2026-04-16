package domain

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCompileAccessLogPattern(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "valid nginx-like pattern",
			pattern: `{remote_addr} {host} - [{time_local}] "{method} {path} {protocol}" {status}`,
		},
		{
			name:    "valid minimal pattern",
			pattern: `{host}{path}`,
		},
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: true,
		},
		{
			name:    "missing host",
			pattern: `{method} {path} {status}`,
			wantErr: true,
		},
		{
			name:    "missing path",
			pattern: `{method} {host} {status}`,
			wantErr: true,
		},
		{
			name:    "duplicate placeholder",
			pattern: `{host} {path} {path}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compiled, err := CompileAccessLogPattern(tc.pattern)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for pattern %q", tc.pattern)
				}
				return
			}

			if err != nil {
				t.Fatalf("CompileAccessLogPattern returned error: %v", err)
			}

			if compiled == nil || compiled.re == nil {
				t.Fatalf("expected compiled regex, got nil")
			}
		})
	}
}

func TestAccessLogPatternParseLineDefaults(t *testing.T) {
	t.Helper()

	pattern, err := CompileAccessLogPattern(`{host}{path}`)
	if err != nil {
		t.Fatalf("CompileAccessLogPattern returned error: %v", err)
	}

	method, host, requestPath, statusCode, ok := pattern.parseLine(`example.com/api/users/123?b=2&a=1`)
	if !ok {
		t.Fatalf("expected line to parse")
	}

	if method != "GET" {
		t.Fatalf("unexpected method: %q", method)
	}

	if statusCode != "200" {
		t.Fatalf("unexpected status code: %q", statusCode)
	}

	if host != "example.com" {
		t.Fatalf("unexpected host: %q", host)
	}

	if requestPath != "/api/users/123?b=2&a=1" {
		t.Fatalf("unexpected path: %q", requestPath)
	}
}

func TestAccessLogPatternParseLineRejectsMalformedLine(t *testing.T) {
	t.Helper()

	pattern, err := CompileAccessLogPattern(`{remote_addr} {host} "{method} {path}" {status}`)
	if err != nil {
		t.Fatalf("CompileAccessLogPattern returned error: %v", err)
	}

	_, _, _, _, ok := pattern.parseLine(`this line does not match`)
	if ok {
		t.Fatalf("expected malformed line to be rejected")
	}
}

func TestParseAccessLog(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "access.log")

	lines := []byte("127.0.0.1 example.com - [01/Jan/2026:00:00:00 +0000] \"GET /api/users/123?b=2&a=1 HTTP/1.1\" 200\n" +
		"127.0.0.1 example.com - [01/Jan/2026:00:00:01 +0000] \"POST /api/users/456 HTTP/1.1\" 201\n" +
		"127.0.0.1 example.com - [01/Jan/2026:00:00:02 +0000] \"GET /api/users/999 HTTP/1.1\" 404\n" +
		"127.0.0.1 example.com - [01/Jan/2026:00:00:03 +0000] \"GET https://example.com/api/orders/550e8400-e29b-41d4-a716-446655440000?a=1&b=2 HTTP/1.1\" 200\n" +
		"malformed line\n")

	if err := os.WriteFile(logPath, lines, 0o600); err != nil {
		t.Fatalf("write temp log file: %v", err)
	}

	results, err := ParseAccessLog(logPath, `{remote_addr} {host} - [{time_local}] "{method} {path} {protocol}" {status}`)
	if err != nil {
		t.Fatalf("ParseAccessLog returned error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("unexpected number of templates: want 3 got %d", len(results))
	}

	byKey := make(map[string]EndpointTemplate, len(results))
	for _, et := range results {
		byKey[et.Method+" "+et.PathTemplate] = et
	}

	getUsers, ok := byKey["GET /api/users/{id}"]
	if !ok {
		t.Fatalf("missing GET /api/users/{id} template")
	}

	if getUsers.Count != 1 {
		t.Fatalf("unexpected count for GET users: %d", getUsers.Count)
	}

	if len(getUsers.Observations) != 1 {
		t.Fatalf("unexpected observations count for GET users: %d", len(getUsers.Observations))
	}

	if getUsers.Observations[0].Source != "logs" {
		t.Fatalf("unexpected source for GET users: %q", getUsers.Observations[0].Source)
	}

	if getUsers.Observations[0].URL != "https://example.com/api/users/123?a=1&b=2" {
		t.Fatalf("unexpected normalized URL for GET users: %q", getUsers.Observations[0].URL)
	}

	postUsers, ok := byKey["POST /api/users/{id}"]
	if !ok {
		t.Fatalf("missing POST /api/users/{id} template")
	}

	if postUsers.Count != 1 {
		t.Fatalf("unexpected count for POST users: %d", postUsers.Count)
	}

	getOrders, ok := byKey["GET /api/orders/{uuid}"]
	if !ok {
		t.Fatalf("missing GET /api/orders/{uuid} template")
	}

	if getOrders.Count != 1 {
		t.Fatalf("unexpected count for GET orders: %d", getOrders.Count)
	}
}

func TestParseAccessLogDefaultsWhenMethodAndStatusAreMissing(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "minimal.log")

	lines := []byte("example.com/api/widgets/100\nexample.com/api/widgets/200\n")
	if err := os.WriteFile(logPath, lines, 0o600); err != nil {
		t.Fatalf("write temp log file: %v", err)
	}

	results, err := ParseAccessLog(logPath, `{host}{path}`)
	if err != nil {
		t.Fatalf("ParseAccessLog returned error: %v", err)
	}

	want := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/api/widgets/{id}",
			Parameters: []Parameter{
				{Name: "id", Type: "int", Source: "inferred"},
			},
			Observations: []ExampleURL{
				{Source: "logs", URL: "https://example.com/api/widgets/100", StatusCode: 200},
				{Source: "logs", URL: "https://example.com/api/widgets/200", StatusCode: 200},
			},
			Count: 2,
		},
	}

	if !reflect.DeepEqual(results, want) {
		t.Fatalf("unexpected endpoints\nwant: %#v\ngot:  %#v", want, results)
	}
}
