package draque_test

import (
	"os"
	"path/filepath"
	"testing"

	draque "github.com/ynori7/draque"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestNew_noOptions(t *testing.T) {
	d := draque.New()
	if d == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_withErrorOnFailure_returnsErrorForBadSource(t *testing.T) {
	d := draque.New(
		draque.WithErrorOnFailure(true),
		draque.WithSwagger("/no/such/file.json"),
	)
	_, err := d.Scan()
	if err == nil {
		t.Fatal("expected error for non-existent swagger file with ErrorOnFailure=true")
	}
}

func TestNew_withErrorOnFailureFalse_skipsFailedSource(t *testing.T) {
	swaggerContent := `{
  "swagger": "2.0",
  "info": {"title": "test", "version": "1"},
  "host": "example.com",
  "basePath": "/",
  "paths": {
    "/users/{id}": {
      "get": {
        "parameters": [{"name": "id", "in": "path", "type": "integer"}],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`
	dir := t.TempDir()
	swaggerFile := writeFile(t, dir, "api.json", swaggerContent)

	d := draque.New(
		draque.WithErrorOnFailure(false),
		draque.WithLogFile("/no/such/file.log", "{host}{path}"),
		draque.WithSwagger(swaggerFile),
	)
	results, err := d.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results despite failed log source")
	}
}

func TestWithSwagger_singleFile(t *testing.T) {
	swaggerContent := `{
  "swagger": "2.0",
  "info": {"title": "test", "version": "1"},
  "host": "example.com",
  "basePath": "/",
  "paths": {
    "/items/{id}": {
      "get": {
        "parameters": [{"name": "id", "in": "path", "type": "integer"}],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`
	dir := t.TempDir()
	swaggerFile := writeFile(t, dir, "api.json", swaggerContent)

	d := draque.New(draque.WithSwagger(swaggerFile))
	results, err := d.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one endpoint template")
	}
}

func TestWithSwaggerDirectory_picksUpJsonAndYaml(t *testing.T) {
	jsonContent := `{
  "swagger": "2.0",
  "info": {"title": "a", "version": "1"},
  "host": "a.example.com",
  "basePath": "/",
  "paths": {
    "/a/{id}": {
      "get": {
        "parameters": [{"name": "id", "in": "path", "type": "integer"}],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`
	yamlContent := `swagger: "2.0"
info:
  title: b
  version: "1"
host: b.example.com
basePath: /
paths:
  /b/{id}:
    get:
      parameters:
        - name: id
          in: path
          type: integer
      responses:
        "200":
          description: ok
`
	dir := t.TempDir()
	writeFile(t, dir, "a.json", jsonContent)
	writeFile(t, dir, "b.yaml", yamlContent)
	writeFile(t, dir, "notes.txt", "not a swagger file")

	d := draque.New(draque.WithSwaggerDirectory(dir))
	results, err := d.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 endpoint templates, got %d", len(results))
	}
}

func TestWithSwaggerDirectory_invalidDir(t *testing.T) {
	d := draque.New(draque.WithSwaggerDirectory("/no/such/directory"))
	_, err := d.Scan()
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestWithLogFile(t *testing.T) {
	lines := "127.0.0.1 example.com - [01/Jan/2026:00:00:00 +0000] \"GET /api/users/123 HTTP/1.1\" 200\n" +
		"127.0.0.1 example.com - [01/Jan/2026:00:00:01 +0000] \"GET /api/users/456 HTTP/1.1\" 200\n"

	dir := t.TempDir()
	logFile := writeFile(t, dir, "access.log", lines)

	d := draque.New(draque.WithLogFile(logFile, `{remote_addr} {host} - [{time_local}] "{method} {path} {protocol}" {status}`))
	results, err := d.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one endpoint template")
	}
}

func TestWithLogDirectory_picksUpAllRegularFiles(t *testing.T) {
	line := "127.0.0.1 example.com - [01/Jan/2026:00:00:00 +0000] \"GET /api/orders/100 HTTP/1.1\" 200\n"
	pattern := `{remote_addr} {host} - [{time_local}] "{method} {path} {protocol}" {status}`

	dir := t.TempDir()
	writeFile(t, dir, "app1.log", line)
	writeFile(t, dir, "app2.log", line)

	d := draque.New(draque.WithLogDirectory(dir, pattern))
	results, err := d.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one endpoint template from log directory")
	}
}

func TestWithLogDirectory_invalidDir(t *testing.T) {
	d := draque.New(draque.WithLogDirectory("/no/such/directory", "{host}{path}"))
	_, err := d.Scan()
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestScan_noSources_returnsNil(t *testing.T) {
	d := draque.New()
	results, err := d.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results for empty Draque, got %v", results)
	}
}

func TestScan_mergesMultipleSources(t *testing.T) {
	makeSwagger := func(host string) string {
		return `{"swagger":"2.0","info":{"title":"t","version":"1"},"host":"` + host + `","basePath":"/","paths":{"/users/{id}":{"get":{"parameters":[{"name":"id","in":"path","type":"integer"}],"responses":{"200":{"description":"ok"}}}}}}`
	}

	dir := t.TempDir()
	a := writeFile(t, dir, "a.json", makeSwagger("a.example.com"))
	b := writeFile(t, dir, "b.json", makeSwagger("b.example.com"))

	d := draque.New(draque.WithSwagger(a), draque.WithSwagger(b))
	results, err := d.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 merged template, got %d", len(results))
	}
}
