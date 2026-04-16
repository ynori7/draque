# Draque
Named after Sir Francis Drake (a.k.a. El Draque), the famous privateer and explorer, this tool allows you to explore an API ecosystem for security testing purposes.

Features:
- Find historical endpoints via the Wayback Machine's API
- Find endpoints via log files
- Find and understand endpoints from Swagger docs
- Take all known endpoints, group them together by route, collect relevant IDs, and infer valid input data for similar endpoints.

## Commands

### Wayback

```bash
go run ./cmd/wayback <domain> [path-prefix]
```

### Access Log Parser

```bash
go run ./cmd/accesslog <log-file> <input-format-pattern>
```

Example pattern (nginx-like):

```text
{remote_addr} {host} - [{time_local}] "{method} {path} {protocol}" {status}
```

The following placeholders have meaning for this scanner:

- host
- path
- method
- status