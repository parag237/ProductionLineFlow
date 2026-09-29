# Warehouse API

This backend is the first implementation slice of the multi-warehouse management system described in the project plan.

## Included

- Config loader using `APP_ENV` and `CONFIG_DIR`
- Gin router skeleton with API health and auth routes
- RBAC actor permission checks
- FSM engine and warehouse flow/domain definitions
- UI screen schema types and navigation helpers
- Docker Compose and config examples for dev/test/prod

## Quick start

```bash
cd warehouse-api
go test ./...
APP_ENV=dev CONFIG_DIR=./config go run ./cmd/api
```

## Next built stages

1. Company bootstrap and migrations
2. Auth login, refresh rotation, and `/me`
3. Users and assignments APIs
4. Full FSM-driven warehouse lifecycle and SDUI flow execution
