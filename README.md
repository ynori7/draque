# Draque
Named after Sir Francis Drake (a.k.a. El Draque), the famous privateer and explorer, this security exploration tool allows you to explore an API ecosystem for security testing purposes.

Features:
- Find historical endpoints via the Wayback Machine's API
- Find endpoints via log files
- Find and understand endpoints from Swagger docs
- Take all known endpoints from all sources, group them together by route, collect relevant IDs, and automatically infer valid input data for similar endpoints.

## Usage

### CLI Tool

The Draque tool is an interactive UI which allows you to scan data from various sources, merge them, and analyze the results.

Usage:

```bash
go build -o draque cmd/draque/main.go
./draque
```

The tool supports the following commands:

- wayback: Add a Wayback Machine source (domain + optional path prefix)
- logs: Add an access log source (file path + format pattern)
- swagger: Add a Swagger/OpenAPI spec source (file path)
- status:  Show configured sources and scan summary
- scan: Fetch and merge all configured sources (with progress)
- analyze: Show statistics about scan results (only available after scanning)
- search: Search endpoints live as you type and view details (only available after scanning)
- export: Export one representative URL per endpoint template to a file (one per line) (only available after scanning)
- reset: Clear all sources and scan data for a fresh start (only available after scanning)

### Library

Draque can be used as a Go library to programmatically scan sources and retrieve merged endpoint templates.

**Example Usage:**

```go
package main

import (
    "fmt"
    "log"

    "github.com/ynori7/draque"
)

func main() {
    d := draque.New(
        // Fetch historical URLs from the Wayback Machine for example.com under /api
        draque.WithWayback("example.com", "/api"),

        // Parse all access log files in a directory; adjust the format pattern to match your log format
        draque.WithLogDirectory("/var/log/myapp", "{host} {method} {path} {status}"),

        // Parse all Swagger/OpenAPI specs (.json, .yaml, .yml) in a directory
        draque.WithSwaggerDirectory("/path/to/swagger/specs"),
    )

    templates, err := d.Scan()
    if err != nil {
        log.Fatal(err)
    }

    for _, t := range templates {
        fmt.Println(t)
    }
}
```

`Scan` fetches all configured sources, normalizes the URLs, and returns a deduplicated slice of `domain.EndpointTemplate` values — one per unique route pattern, with inferred path parameters substituted back in.

**Other options:**

| Option | Description |
|---|---|
| `WithWayback(domain, prefix)` | Fetch URLs from the Wayback Machine for the given domain and optional path prefix. |
| `WithLogFile(path, format)` | Parse a single access log file using the given format pattern. |
| `WithLogDirectory(path, format)` | Parse all regular files in a directory as access logs. |
| `WithSwagger(path)` | Parse a single Swagger/OpenAPI spec file (JSON or YAML). |
| `WithSwaggerDirectory(path)` | Parse all `.json`, `.yaml`, and `.yml` files in a directory as Swagger specs. |
| `WithErrorOnFailure(true)` | Return immediately on the first source failure instead of skipping it. |