# ProductionLineFlow API

This backend is the first implementation slice of the ProductionLineFlow multi-warehouse management system described in the project plan.

## Included

- Config loader using `APP_ENV` and `CONFIG_DIR`
- CORS origins configurable with the comma-separated `ALLOWED_ORIGINS` environment variable
- Gin router skeleton with API health and auth routes
- RBAC actor permission checks
- FSM engine and warehouse flow/domain definitions
- UI screen schema types and navigation helpers
- Docker Compose and config examples for dev/test/prod

## Quick start

```bash
cd productionlineflow-api
go test ./...
APP_ENV=dev CONFIG_DIR=./config go run ./cmd/api
```

For deployment, set `ALLOWED_ORIGINS` on the API service to the frontend's exact origin, such as `https://your-site.netlify.app` (no trailing slash). Multiple origins can be comma-separated.

## Next built stages

1. Product Owner platform access and atomic company/Super Admin onboarding with migrations
2. Auth login, refresh rotation, and `/me`
3. Users and assignments APIs
4. Full FSM-driven warehouse lifecycle and SDUI flow execution
