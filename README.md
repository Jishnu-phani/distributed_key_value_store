# Distributed Key-Value Store — Single-node (Part A)

This folder contains a minimal single-node HTTP key-value store used as the foundation for the distributed project.

Run locally:

```bash
go run main.go
```

Endpoints:
- `PUT /v1/kv/{key}` — set the value (raw body)
- `GET /v1/kv/{key}` — get the value (returns 200 + raw body or 404)
- `DELETE /v1/kv/{key}` — delete the key
- `GET /health` — health check

Example `curl` commands:

```bash
curl -X PUT http://localhost:8080/v1/kv/foo -d 'bar'
curl http://localhost:8080/v1/kv/foo
curl -X DELETE http://localhost:8080/v1/kv/foo
```

Next steps:
- Add Dockerfile for containerization (`A1.3`)
- Add basic integration test harness (scripts/test.sh)
