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

TODO