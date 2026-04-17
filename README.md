# Draque
Named after Sir Francis Drake (a.k.a. El Draque), the famous privateer and explorer, this tool allows you to explore an API ecosystem for security testing purposes.

Features:
- Find historical endpoints via the Wayback Machine's API
- Find endpoints via log files
- Find and understand endpoints from Swagger docs
- Take all known endpoints, group them together by route, collect relevant IDs, and infer valid input data for similar endpoints.

## Commands

### Draque

The Draque tool is an interactive UI which allows you to scan data from various sources, merge them, and analyze the results.

Usage:

```bash
go build -o draque cmd/draque/main.go
./draque
```