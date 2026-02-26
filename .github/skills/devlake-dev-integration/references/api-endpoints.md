# API Endpoint Reference

## Generic Typed Helpers

All API calls use generic helpers in `internal/devlake/client.go`:

| Helper | HTTP Method | Success Codes |
|--------|------------|---------------|
| `doPost[T]` | POST | 200, 201 |
| `doGet[T]` | GET | 200 |
| `doPut[T]` | PUT | 200, 201 |
| `doPatch[T]` | PATCH | 200 |

Usage pattern:

```go
result, err := doPost[Connection](c, "/plugins/github/connections", req)
```

All helpers: marshal request → send → check status → unmarshal response into `*T`.

## API Endpoint Patterns

DevLake REST API follows consistent URL patterns:

| Operation | Pattern | Example |
|-----------|---------|---------|
| List connections | `GET /plugins/{plugin}/connections` | `/plugins/github/connections` |
| Create connection | `POST /plugins/{plugin}/connections` | `/plugins/gh-copilot/connections` |
| Test connection | `POST /plugins/{plugin}/test` | `/plugins/github/test` |
| Test saved connection | `POST /plugins/{plugin}/connections/{id}/test` | `/plugins/github/connections/1/test` |
| Scope configs | `GET/POST /plugins/{plugin}/connections/{id}/scope-configs` | |
| Upsert scopes | `PUT /plugins/{plugin}/connections/{id}/scopes` | |
| Projects | `GET/POST /projects` | `/projects/MyProject` |
| Blueprints | `PATCH /blueprints/{id}`, `POST /blueprints/{id}/trigger` | |
| Pipelines | `GET /pipelines/{id}` | |
| Health | `GET /ping` | |
