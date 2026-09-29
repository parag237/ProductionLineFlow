# ProductionLineFlow Platform User Login and Onboarding Plan

## Goal

Add a secure platform authentication flow for Product Owners and connect it to a platform operations UI. A platform user must be able to sign in independently of tenant users, receive platform-only permissions, create a company with its initial Super Admin, and never receive implicit tenant access.

This plan is intentionally separate from the tenant login implementation in `login_implementation_plan.md`.

## Current State

- `platform_users`, `platform_roles`, `platform_role_permissions`, and `platform_user_role_assignments` exist in the initial database migration.
- The `product_owner` platform role and `companies.create` permission are seeded.
- `internal/rbac/actor.go` contains `PlatformActor.Can`, but platform actors are not loaded from PostgreSQL.
- `internal/auth/jwt.go` only creates tenant access tokens containing `company_id`.
- `internal/auth/postgres_repository.go` only queries tenant users and tenant refresh tokens.
- `internal/server/router.go` has no `/platform/*` routes or platform middleware.
- The web client currently renders only the tenant login flow and authenticated tenant workspace.
- The existing `refresh_tokens` table references `users`, so platform refresh tokens require a separate table or a carefully designed schema migration.

## Security Boundary

Platform and tenant authentication must remain separate:

| Concern | Tenant | Platform |
|---|---|---|
| Login | `/api/v1/auth/login` | `/api/v1/platform/auth/login` |
| User table | `users` | `platform_users` |
| Refresh table | `refresh_tokens` | `platform_refresh_tokens` |
| Access token subject | `user_id` + `company_id` | `platform_user_id`, no company ID |
| Actor | `Actor` | `PlatformActor` |
| Permission source | company roles and assignments | platform roles and permissions |
| Company access | one token company | no tenant membership by default |
| Login UI | tenant workspace login | platform operations login |

A tenant token must never authorize a platform route. A platform token must never authorize a tenant route. Do not use the presence of a role name alone as authorization.

## Target Platform User Flow

1. Product Owner opens the platform login page.
2. UI submits email and password to `/api/v1/platform/auth/login`.
3. API finds the active row in `platform_users` and verifies the Argon2id password.
4. API loads platform role assignments and effective permissions.
5. API issues a short-lived platform access JWT and an opaque platform refresh token.
6. Web stores the access token in memory and the refresh token in an httpOnly platform cookie.
7. UI calls `/api/v1/platform/me` to hydrate the Product Owner session.
8. Platform navigation shows company onboarding only when `companies.create` is present.
9. Product Owner opens the company onboarding form.
10. API creates the company, four tenant system roles, default role permissions, the initial Super Admin, and the Super Admin assignment in one PostgreSQL transaction.
11. API returns the company and non-sensitive Super Admin information, never a password or platform-to-tenant membership.
12. Logout revokes the platform refresh token and clears only the platform session cookie.

## Database Changes

Create a new migration rather than changing the already-applied initial migration.

### Platform Refresh Tokens

Add `platform_refresh_tokens`:

```sql
CREATE TABLE platform_refresh_tokens (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    platform_user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by BIGINT REFERENCES platform_refresh_tokens(id),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX platform_refresh_tokens_user_active
    ON platform_refresh_tokens (platform_user_id)
    WHERE revoked_at IS NULL;
```

Use a separate table so a tenant refresh token can never be looked up as a platform token.

### Operator Bootstrap

Add a one-time operator bootstrap command or documented deployment task:

- Requires an operator-provided email, name, and password.
- Hashes the password with the existing Argon2id helper.
- Inserts or updates a `platform_users` row.
- Assigns the existing `product_owner` system role.
- Never seeds a default password or default Product Owner account in a migration.
- Refuses to run in production without explicit bootstrap input.

Recommended command shape:

```text
productionlineflow operator bootstrap-product-owner
```

Do not expose this operation as a public HTTP endpoint.

## API Contract

### Platform Login

`POST /api/v1/platform/auth/login`

Request:

```json
{
  "email": "operator@example.com",
  "password": "..."
}
```

Success response:

```json
{
  "access_token": "jwt",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": 1,
    "name": "Platform Operator",
    "email": "operator@example.com"
  },
  "permissions": ["companies.create"]
}
```

Set the refresh token only as an httpOnly cookie named separately from the tenant cookie, for example `productionlineflow_platform_refresh`.

Failure behavior:

- `400` for malformed or missing fields.
- `401` with `invalid_platform_credentials` for unknown, wrong, or inactive credentials.
- `429` for rate-limit violations.
- `500` with no database, password, or token details.

### Platform Refresh

`POST /api/v1/platform/auth/refresh`

- Read only the platform refresh cookie or an explicitly documented mobile platform token header.
- Hash before lookup.
- Lock the token row with `FOR UPDATE`.
- Reject expired or revoked tokens.
- Rotate the old token and set `replaced_by` in one transaction.
- Reuse of a rotated token revokes all active platform tokens for that platform user.
- Issue a platform access token with current `perm_version`.

### Platform Logout

`POST /api/v1/platform/auth/logout`

- Revoke the presented platform refresh token.
- Clear the platform cookie.
- Do not revoke tenant refresh tokens because a platform user is not a tenant user.
- Make repeated logout safe.

### Platform Me

`GET /api/v1/platform/me`

Requires platform authentication and returns:

- Platform user identity.
- Platform role slugs.
- Effective platform permission keys.
- Current `perm_version`.

### Company Onboarding

`POST /api/v1/platform/companies`

Requires platform permission `companies.create`.

Request:

```json
{
  "name": "Acme Manufacturing",
  "slug": "acme",
  "super_admin": {
    "name": "Jane Admin",
    "email": "jane@acme.test",
    "password": "temporary-password"
  }
}
```

Transactional behavior:

1. Lock or validate the company slug uniqueness.
2. Insert the company.
3. Insert the four tenant system roles: `super_admin`, `admin`, `manager`, `worker`.
4. Attach the default permission matrix to those roles.
5. Insert the initial Super Admin with an Argon2id password hash.
6. Assign the Super Admin role with no warehouse ID.
7. Commit all changes together.
8. Return company and non-sensitive user fields only.

A failure at any step must roll back the entire onboarding operation.

## Backend Implementation Sequence

### 1. Platform Domain Types

Add separate types rather than reusing tenant `User` or tenant `Session` blindly:

- `PlatformUser`
- `PlatformPermission`
- `PlatformSession`
- `PlatformClaims`
- `PlatformRepository`
- `PlatformAuthService`

Use `int64` IDs and keep `CompanyID` absent from platform claims.

### 2. Platform JWT Claims

Extend the JWT service with explicit platform methods or a token-kind abstraction:

```text
platform_user_id
perm_version
token_type = platform_access
issuer
subject
issued_at
expires_at
jti
```

Rules:

- Platform claims must not contain `company_id`.
- Tenant validation must reject `platform_access` tokens.
- Platform validation must reject `tenant_access` tokens.
- Enforce the expected signing algorithm and issuer.
- Use a distinct token type and, if practical, a distinct signing key configuration.

### 3. Platform Repository

Add queries for:

- Find active platform user by email.
- Find active platform user by ID.
- Load platform roles and permissions.
- Insert and rotate platform refresh tokens.
- Revoke one platform token.
- Revoke all active platform tokens for reuse detection or deactivation.
- Increment `platform_users.perm_version` when platform assignments change.

Never join platform authorization through tenant assignment tables.

### 4. Platform Auth Service

Implement:

- Credential verification using the existing Argon2id helper.
- Generic invalid-credential responses.
- Platform access-token issuance.
- Opaque refresh-token generation and SHA-256 storage.
- Transactional refresh rotation.
- Refresh reuse detection.
- Logout and token revocation.

Tests:

- Valid Product Owner login succeeds.
- Wrong password, unknown email, and inactive user have the same response shape.
- Tenant credentials cannot authenticate on the platform endpoint.
- Platform tokens contain no company ID.
- Platform refresh tokens cannot be used with tenant refresh routes.
- Reused platform refresh tokens revoke the remaining platform session chain.

### 5. Platform Middleware

Add `RequirePlatformAuth`:

1. Read the Bearer token.
2. Validate platform claims and token type.
3. Load the platform user and verify active status.
4. Compare `perm_version`.
5. Load platform permissions.
6. Attach `PlatformActor` to request context.
7. Continue only for valid platform sessions.

Add `RequirePlatformPermission("companies.create")` to the company onboarding route. The handler must check the actor again at the service boundary so authorization does not depend only on router wiring.

### 6. Router Wiring

Add public platform routes:

```text
POST /api/v1/platform/auth/login
POST /api/v1/platform/auth/refresh
POST /api/v1/platform/auth/logout
```

Add protected platform routes:

```text
GET  /api/v1/platform/me
POST /api/v1/platform/companies
```

Keep platform and tenant route groups separate in Gin. Avoid sharing middleware that assumes `company_id` exists.

### 7. Company Onboarding Service

Create a dedicated service with a transaction-aware repository:

- Validate company name and slug format.
- Validate email and password length.
- Check duplicate slug and initial-admin email within the new company.
- Seed role and permission assignments from a versioned default matrix.
- Prevent platform users from being inserted into tenant `users`.
- Return typed conflict errors for duplicate company slugs.

Integration tests must verify rollback by forcing a failure after role creation and confirming that no company, roles, or user remain.

## Web UI Plan

Use a distinct platform route and session namespace in the existing web app.

### Routes and Session Modes

Add:

- `/platform/login`
- `/platform` or `/platform/companies`
- `/platform/companies/new`

Use a platform auth state separate from tenant auth state:

```text
unknown -> unauthenticated -> authenticating -> authenticated -> refreshing -> signed_out
```

Do not let tenant session restoration call platform refresh, and do not let platform session restoration call tenant refresh.

### Platform Login Screen

Fields:

- Email
- Password

States:

- Initial loading/session restoration
- Submitting
- Invalid credentials
- Rate limited
- Network failure
- Authenticated redirect

UX requirements:

- Generic error text for invalid credentials.
- Password is cleared after failed submission.
- Email remains populated after a failed submission.
- Form is keyboard accessible.
- Focus moves to the error on failure.
- Refresh cookie is never readable by JavaScript.

### Platform Shell

Show:

- Platform identity and email.
- Product Owner permission summary.
- Company list or onboarding entry point.
- Sign-out action.
- Clear platform visual label so a user knows they are outside a tenant workspace.

Navigation should be generated from `/platform/me` and permission keys. The UI may hide unavailable actions, but the API remains authoritative.

### Company Onboarding UI

`/platform/companies/new` should be a two-step or review flow:

1. Company details: name and slug.
2. Initial Super Admin details: name, email, password, confirm password.
3. Review and submit.
4. Success screen with company identity and the Super Admin's non-sensitive fields.

Client-side validation:

- Required fields.
- Slug format and length.
- Email format.
- Password minimum length.
- Password confirmation match.

Server-side validation remains authoritative.

Never show the submitted password after success and never include it in URL state, logs, analytics, or error messages.

### API Client Changes

Add platform-specific client methods:

- `platformLogin`
- `platformRefresh`
- `platformLogout`
- `getPlatformMe`
- `createCompany`

Keep platform access token in memory and use a separate refresh-cookie path. Add a single-refresh retry guard so a failed platform refresh cannot loop indefinitely.

## Mobile Follow-up

When mobile platform operations are required:

- Add a separate platform auth store using secure storage.
- Use platform endpoints and platform token types only.
- Share validation schemas with web.
- Keep tenant and platform navigation stacks separate.
- Add a confirmation step before company creation.

## Testing Plan

### Backend Unit Tests

- Platform password verification.
- Typed platform JWT creation and rejection of tenant token types.
- Platform refresh rotation and reuse detection.
- Permission checks for `companies.create`.
- Generic credential errors.

### Backend Integration Tests

- Product Owner login -> platform `/me`.
- Platform token -> company creation.
- Tenant token -> platform route returns `401`.
- Platform token -> tenant route returns `401`.
- Company onboarding commits all expected rows.
- Company onboarding rolls back on any failure.
- Duplicate company slug returns `409`.
- Inactive Product Owner cannot authenticate.

### Web Tests

- Platform login success.
- Invalid platform credentials.
- Reload restores platform session.
- Platform refresh failure returns to platform login.
- Permission-free platform user cannot see or invoke onboarding.
- Company onboarding validation and success.
- Password confirmation is client-only and never sent as a separate server field.
- Platform logout clears platform state without affecting tenant state.

## Delivery Phases

### Phase 1: Schema and Bootstrap

- Add platform refresh-token migration.
- Add one-time Product Owner bootstrap command.
- Add seed/test fixture documentation.

Checkpoint: a test Product Owner exists only through explicit bootstrap and platform refresh rows can be created.

### Phase 2: Platform Auth Core

- Add platform repository.
- Add platform JWT claims.
- Add platform service and rotation logic.
- Add unit and integration tests.

Checkpoint: platform login, refresh, logout, and `/platform/me` pass without tenant authentication.

### Phase 3: Platform Middleware and Onboarding

- Add platform router group and middleware.
- Implement transactional company onboarding.
- Add authorization checks at middleware and service layers.

Checkpoint: Product Owner can create a company; non-authorized platform users receive `403`; tenant tokens receive `401`.

### Phase 4: Web Platform UI

- Add platform session client.
- Add `/platform/login`.
- Add platform shell and company onboarding form.
- Add permission-aware navigation and error states.

Checkpoint: browser test covers login -> platform dashboard -> company creation -> logout.

### Phase 5: Hardening

- Add rate limiting, audit events, metrics, and secure cookie review.
- Document bootstrap and emergency revocation procedures.
- Verify tenant/platform token separation with security tests.

## Acceptance Criteria

- Product Owners authenticate through a platform-only endpoint.
- Platform access tokens contain no tenant company ID.
- Tenant and platform refresh tokens are stored separately and cannot be exchanged.
- Platform middleware loads `PlatformActor` and checks permissions.
- Only `companies.create` can create companies.
- Company onboarding is fully transactional.
- Initial Super Admin credentials are hashed and never returned.
- Platform and tenant web sessions are isolated.
- Platform UI supports login, refresh, logout, permission-aware navigation, and company creation.
- No default Product Owner credentials are shipped in migrations or source code.
