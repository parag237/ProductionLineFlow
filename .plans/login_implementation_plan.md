# ProductionLineFlow Login and Authorization Implementation Plan

## Goal

Replace the placeholder authentication routes with a production-ready tenant login flow and connect the web and mobile clients to it. The finished flow must authenticate a user within a company, issue and rotate tokens, load effective role permissions, protect API routes, and render the correct authenticated UI.

The plan covers tenant users first and keeps platform Product Owner authentication separate. It assumes the initial schema in `productionlineflow-api/db/migrations/000001_initial.up.sql` uses auto-incrementing `BIGINT` identifiers.

## Current Starting Point

- `productionlineflow-api/internal/server/router.go` contains placeholder handlers for login, refresh, logout, and `/me`.
- `productionlineflow-api/internal/auth/jwt.go` signs and parses JWTs but does not yet enforce issuer, algorithm, expiry, or typed claims.
- `productionlineflow-api/internal/rbac/actor.go` contains the permission-checking shape, but assignments are not loaded from PostgreSQL.
- The migration creates users, companies, roles, permissions, assignments, and refresh-token tables.
- `productionlineflow-frontend/apps/web/src/App.tsx` is only a dashboard placeholder.
- Shared frontend packages exist as scaffolding but do not yet provide the login/session flow.

## Target User Flow

1. User opens the web or mobile app.
2. Client checks for an existing authenticated session.
3. If no valid session exists, the client renders the login screen.
4. User submits company slug, email, and password.
5. API validates input, finds the user in that company, verifies the Argon2id password, and rejects inactive or unknown users with the same generic authentication error.
6. API creates and stores a hashed opaque refresh token, signs a short-lived access JWT, loads the user's effective permissions, and returns the session response.
7. Client stores the access token in memory and stores the refresh token in an httpOnly cookie on web or secure storage on mobile.
8. Client calls `/me` to hydrate the current user, assignments, permission map, and navigation.
9. Protected API requests attach the access token.
10. On access-token expiry, the client calls refresh once, retries the original request once, and redirects to login if refresh fails.
11. Logout revokes the refresh token and clears client state.

## API Contract

### Tenant Login

`POST /api/v1/auth/login`

Request:

```json
{
  "company_slug": "acme",
  "email": "user@example.com",
  "password": "secret"
}
```

Success response, `200`:

```json
{
  "access_token": "jwt",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": 1,
    "name": "Jane User",
    "email": "user@example.com",
    "company_id": 1,
    "company_slug": "acme"
  },
  "permissions": {
    "company": ["warehouse.view"],
    "warehouses": {}
  }
}
```

The refresh token is never returned in JSON. The web response sets it as an httpOnly, Secure cookie with an appropriate SameSite policy. The mobile API response may return the opaque refresh token through a mobile-specific transport or the shared client may receive it from a secure response field; do not log it.

Failure responses:

- `400`: malformed JSON or missing fields.
- `401` with stable code `invalid_credentials`: unknown company, unknown email, wrong password, inactive company/user, or invalid credential combination.
- `429`: rate limit exceeded.
- `500`: internal error without database or password details.

### Refresh

`POST /api/v1/auth/refresh`

- Read the refresh token from the web cookie or mobile secure-storage request.
- Hash the presented token with SHA-256 before lookup.
- Lock the refresh-token row with `FOR UPDATE`.
- Reject expired or revoked tokens.
- Rotate the token in one transaction: revoke old row, insert new row, set `replaced_by`.
- If a revoked token is reused and has `replaced_by`, revoke the user's active refresh-token chain and return `401`.
- Issue a new access JWT with the current `perm_version`.

### Logout

`POST /api/v1/auth/logout`

- Revoke the presented refresh token transactionally.
- Clear the web cookie.
- Treat an already-revoked token as an idempotent success where possible.
- Do not require a valid access token if a valid refresh token is presented; allow clients to recover from access-token expiry.

### Current User

`GET /api/v1/me`

Requires tenant authentication and returns:

- User and company identity.
- Active assignments with role slug, role scope, and warehouse ID.
- Effective company-wide permissions.
- Effective per-warehouse permissions.
- `perm_version` used to detect stale authorization state.

### Platform Login

Implement separately after the tenant flow:

- `POST /api/v1/platform/auth/login`
- `POST /api/v1/platform/auth/refresh`
- `POST /api/v1/platform/auth/logout`

Platform tokens identify `platform_user_id` and never contain `company_id`. Platform middleware must load `PlatformActor` and must not reuse tenant actor checks.

## Backend Implementation Sequence

### 1. Database Access Foundation

Add the database package and connection lifecycle:

- Add `pgx` pool dependency.
- Load the configured DSN and max connections.
- Add context-aware pool creation, ping, and shutdown.
- Add migration execution for local/dev startup or document the external migration command.
- Add repository interfaces for users, companies, roles/permissions, and refresh tokens.
- Use the `BIGINT` database IDs consistently in Go types. Update the current string ID fields in RBAC/auth boundaries when those values become database-backed.
- Keep every tenant query scoped by `company_id` from the authenticated request, never from a client-provided company ID.

Tests:

- Pool configuration and shutdown.
- Query scans for `BIGINT` IDs.
- Company/email lookup proves the same email can authenticate in two companies.

### 2. Password and Credential Services

Add an auth service responsible for credential decisions:

- Hash new passwords with Argon2id using documented parameters.
- Verify passwords without revealing whether the company, email, or password was wrong.
- Enforce the configured minimum password length on create/reset, not as a login-only rule.
- Reject inactive users.
- Use a constant-time-safe verification path and avoid logging passwords or hashes.
- Normalize email consistently with the database `CITEXT` behavior; do not alter the original display value unnecessarily.

Tests:

- Valid password succeeds.
- Wrong password fails.
- Inactive user fails.
- Empty/short login input returns `400`.
- Unknown company/email returns the same `401` shape as a wrong password.

### 3. Strict JWT Service

Refactor `internal/auth/jwt.go`:

- Define typed access-token claims: token ID, user ID, company ID, permission version, issuer, issued-at, and expiry.
- Use the configured issuer and a fixed expected signing algorithm.
- Reject missing, malformed, expired, or algorithm-mismatched tokens.
- Parse numeric IDs safely as `int64`.
- Do not put roles or a full permission list in the JWT.
- Keep access TTL short, using `auth.access_ttl_min`.
- Add separate typed claims for platform tokens if platform auth is implemented in the same package.

Tests:

- Valid token parses.
- Expired token fails.
- Wrong secret fails.
- Wrong issuer fails.
- Algorithm substitution fails.
- Missing required claims fails.
- Tenant and platform claims cannot be confused.

### 4. Refresh Token Service

Implement opaque refresh-token issuance and rotation:

- Generate cryptographically random token bytes.
- Store only a SHA-256 hash in `refresh_tokens`.
- Store expiry, user agent, company ID, and replacement relation.
- Use a transaction and row lock during rotation.
- Revoke all outstanding tokens when a user is deactivated or a reuse signal is detected.
- Add cleanup for expired rows.

Tests:

- Token hash is stored, never plaintext.
- Rotation invalidates the old token.
- Concurrent refresh attempts allow only one successful rotation.
- Reuse of a revoked token revokes the chain.
- Logout revokes the presented token.

### 5. Tenant Authentication Middleware

Add middleware under the server/platform middleware boundary:

- Read `Authorization: Bearer <access-token>`.
- Validate the JWT.
- Load `user_id`, `company_id`, and `perm_version` into request context.
- Reject missing or malformed credentials with `401` and a stable error envelope.
- Load the user and verify active status.
- Compare the JWT permission version to the current user version. Return `401` with `permissions_changed` when stale, or reload permissions according to the chosen cache strategy.
- Never accept `company_id` from a request body, query, or route parameter as the tenant source.

Add separate platform middleware for `/api/v1/platform/*`.

Tests:

- Protected route rejects anonymous requests.
- Valid tenant token reaches the handler.
- Platform token cannot access tenant routes.
- Tenant token cannot access platform routes.
- Stale permission version is rejected or refreshed consistently.

### 6. Actor and Permission Loading

Implement a permission repository/service that joins:

`user_role_assignments -> roles -> role_permissions -> permissions`

Rules:

- Company-scoped assignments apply to every warehouse in the company.
- Warehouse-scoped assignments apply only to their warehouse.
- Effective access is the union of all assignments.
- Inactive users receive no actor.
- Assignment rows must match the user's company and role company.
- Role scope controls whether `warehouse_id` is required.
- Super Admin and Admin permissions come from role permissions, never from a hard-coded role check.
- Bump `users.perm_version` whenever assignments or role permissions change.
- Cache by user ID plus permission version if Redis is available; invalidate on version change.

Update `internal/rbac/actor.go` to use database-compatible integer IDs while preserving `Can(permission, warehouseID)` semantics.

Tests:

- Company permission grants all warehouses.
- Warehouse permission does not leak to another warehouse.
- Multiple assignments union correctly.
- Removing a role removes its permissions after version invalidation.
- No role string alone authorizes a request.

### 7. Auth Handlers and Router Wiring

Replace placeholder closures in `internal/server/router.go` with injected dependencies:

- `Auth.Login`
- `Auth.Refresh`
- `Auth.Logout`
- `Me.Get`
- Tenant auth middleware on `/me`, navigation, screens, flows, and warehouse routes.
- Public access only for health, tenant login, tenant refresh, and platform login/refresh.
- Add request ID, structured logging, CORS, rate limiting, and request body size limits.
- Use one JSON error envelope, for example `{ "error": { "code": "invalid_credentials", "message": "Invalid credentials" } }`.
- Do not return SQL errors, password errors, JWT internals, or stack traces.

Handler tests should use mocked services and cover success, validation, unauthorized, forbidden, conflict, and internal failure responses.

### 8. Rate Limiting and Security Controls

Add Redis-backed limits at minimum for:

- Login by IP and company/email combination.
- Refresh by user/token fingerprint.
- Password reset/change endpoints.

Also add:

- Secure cookie attributes in production.
- No access or refresh tokens in application logs.
- Audit events for login success/failure, refresh reuse, logout, deactivation, and password changes.
- Generic login failure timing and response.
- CORS restricted to configured origins.

## Frontend Implementation Plan

### Shared Packages

Add or complete these shared pieces:

- `packages/api-client`: typed auth endpoints, bearer transport, refresh transport, and normalized API errors.
- `packages/schemas`: Zod schemas for login request, login response, `/me`, and error envelopes.
- `packages/permissions`: `can(permission, warehouseId)` and navigation filtering helpers.
- `packages/sdui-core`: authenticated session types and a common auth state machine.
- `packages/ui-tokens`: form, alert, loading, and focus-state tokens shared by web/mobile.

The client auth state should have explicit states:

`unknown -> unauthenticated -> authenticating -> authenticated -> refreshing -> signed_out`.

Avoid storing permissions as the source of truth for backend authorization; they are only UI hints.

### Web UI

Replace the scaffold in `apps/web/src/App.tsx` with routes/views for:

- `/login`: company slug, email, password, submit, loading state, generic error state.
- Authenticated application shell: navigation, current user menu, company identity, logout.
- Protected route boundary that waits for session restoration before redirecting.
- Unauthorized view for authenticated users lacking a permission.
- Session-expired handling that returns the user to login without an infinite refresh loop.

Web storage rules:

- Keep access token in memory where practical.
- Use an httpOnly refresh cookie; JavaScript must not read it.
- On app startup call `/auth/refresh` and then `/me`.
- On `401 permissions_changed`, refetch `/me` and rebuild navigation.
- On refresh failure, clear in-memory state and redirect to `/login`.

Login UX requirements:

- Disable duplicate submits.
- Preserve the company slug and email after a failed password attempt, but never preserve the password.
- Use accessible labels, keyboard submission, focus management, and screen-reader error announcements.
- Show a generic authentication error rather than identifying which credential was wrong.
- Render network, rate-limit, and server errors distinctly where safe.
- Add responsive layouts for narrow mobile-width browsers.

### Mobile UI

In `apps/mobile` add:

- Native login screen with company slug, email, and password fields.
- Secure-store refresh token persistence.
- In-memory access token.
- App-start session restoration.
- Navigation reset after logout or refresh failure.
- Offline-aware error state that does not erase a valid cached session unnecessarily.
- Shared Zod validation and auth state transitions from `sdui-core`.

### Authenticated Navigation

After `/me` succeeds:

- Fetch or build `/navigation` from effective permissions.
- Hide unavailable navigation items as a usability hint only.
- Keep route guards and API authorization active even when an item is hidden.
- Provide a warehouse switcher based on `warehouse.view` access.
- Recompute navigation after permission-version changes.

## Delivery Phases and Checkpoints

### Phase 1: Contracts and Dependencies

- Finalize JSON schemas and error envelope.
- Add pgx, Argon2id, Redis, and test dependencies.
- Define service/repository interfaces.
- Confirm integer ID types across Go auth/RBAC models.

Checkpoint: package compilation and schema-generated client types compile.

### Phase 2: Database and Repositories

- Connect PostgreSQL.
- Add user/company lookup, refresh-token, assignment, and permission queries.
- Add transaction helpers and row-lock queries.

Checkpoint: repository integration tests run against PostgreSQL and prove tenant isolation.

### Phase 3: Authentication Core

- Implement Argon2id credential verification.
- Harden JWT claims and validation.
- Implement refresh token hashing, rotation, revocation, and reuse detection.

Checkpoint: auth service unit tests pass, including concurrency-sensitive refresh tests.

### Phase 4: API and Middleware

- Implement handlers.
- Wire router dependencies.
- Add tenant and platform middleware.
- Add `/me`, permission loading, and version invalidation.
- Add rate limiting and structured errors.

Checkpoint: API integration tests cover the complete login -> `/me` -> protected request -> refresh -> logout path.

### Phase 5: Web Client

- Implement API client and auth state machine.
- Build login and authenticated shell.
- Add refresh-once behavior and protected routes.
- Add permission-aware navigation and logout.

Checkpoint: browser tests cover successful login, invalid credentials, reload restoration, expired access token, logout, and forbidden navigation.

### Phase 6: Mobile Client

- Implement secure storage and native login.
- Share schemas and auth state behavior.
- Add session restoration and navigation reset.

Checkpoint: mobile tests cover login, restart restoration, logout, and refresh failure.

### Phase 7: Hardening and Documentation

- Add audit events and operational metrics.
- Verify cookie/CORS/rate-limit production settings.
- Document migration, local database startup, test credentials/bootstrap, and environment variables.
- Add OpenAPI/JSON Schema documentation for auth endpoints.
- Run Go tests, frontend typecheck/lint/tests, API integration tests, and browser/mobile smoke tests.

## Acceptance Criteria

- A valid tenant user can log in with company slug, email, and password.
- The same email can authenticate in different companies with the correct company slug.
- Passwords are Argon2id hashes and never appear in responses or logs.
- Access tokens are short-lived JWTs with strict validation and no role claim.
- Refresh tokens are opaque, hashed at rest, rotated, revocable, and reuse-detecting.
- Every protected request is tenant-scoped by the token and checks effective permissions.
- Warehouse-scoped permissions cannot access another warehouse.
- Role and permission changes invalidate stale authorization state.
- Web and mobile clients restore sessions, refresh once, handle logout, and render permission-aware navigation.
- Platform Product Owner authentication remains isolated from tenant authentication.
- No placeholder auth handlers remain in the router.
