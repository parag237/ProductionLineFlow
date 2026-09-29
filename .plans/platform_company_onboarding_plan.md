# Platform Company and Super Admin Onboarding Plan

## Objective

Allow an authenticated platform Product Owner to create a tenant company and its initial Super Admin from the platform UI. The operation must be authorized by `companies.create`, transactional, tenant-isolated, and immediately usable by signing in through the tenant login flow.

## Existing Implementation

The first vertical slice is already present:

- Platform login: `POST /api/v1/platform/auth/login`
- Platform refresh/logout: `/api/v1/platform/auth/refresh` and `/api/v1/platform/auth/logout`
- Platform session: `GET /api/v1/platform/me`
- Protected onboarding endpoint: `POST /api/v1/platform/companies`
- Platform permission middleware for `companies.create`
- Transactional company repository that creates:
  - Company
  - `super_admin`, `admin`, `manager`, and `worker` roles
  - Default role permissions
  - Initial Super Admin user
  - Super Admin role assignment
- Web platform login and `/platform` workspace
- Web company onboarding form and success state
- Development Product Owner fixture in `db/seeds/dev.sql`

## Security Rules

1. Only a platform actor with `companies.create` can access onboarding.
2. Tenant tokens must return `401` on platform routes.
3. Platform tokens must not contain `company_id`.
4. The Product Owner must not be inserted into the tenant `users` table or assigned tenant permissions.
5. The initial Super Admin password must be hashed with Argon2id.
6. Passwords must never appear in logs, responses, URL state, or frontend success state.
7. Company creation must use the company name and slug from the request only after validation.
8. The request must not accept `company_id` or tenant permissions from the client.
9. All company, role, permission, user, and assignment rows must commit in one transaction.
10. Any failure must roll back the entire onboarding operation.

## API Contract

### Request

`POST /api/v1/platform/companies`

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

### Success

Return `201 Created`:

```json
{
  "id": 1,
  "slug": "acme",
  "name": "Acme Manufacturing",
  "super_admin_id": 2,
  "super_admin_name": "Jane Admin",
  "super_admin_email": "jane@acme.test"
}
```

Do not return the password, password hash, refresh token, or tenant access token.

### Error Contract

- `400 invalid_request`: malformed body, invalid slug, missing fields, invalid email, or short password.
- `401 unauthorized`: missing or invalid platform access token.
- `403 forbidden`: authenticated platform user lacks `companies.create`.
- `409 company_creation_failed`: duplicate slug, duplicate initial admin email, or transaction conflict.
- `500 internal_error`: unexpected database or infrastructure failure without internal details.

## Backend Work Plan

### 1. Validate Inputs

Add typed request validation:

- Company name is required and trimmed.
- Slug is lowercase, unique, and matches `^[a-z0-9]+(?:-[a-z0-9]+)*$`.
- Super Admin name is required.
- Email is normalized consistently with `CITEXT`.
- Password meets `auth.min_password_length`.
- Confirm-password is a client-only field and is never required by the API.

### 2. Authorize at Two Boundaries

Keep both checks:

- Router middleware checks `PlatformActor.Can("companies.create")`.
- Company service checks the permission or receives an already-authorized platform actor and verifies it before opening the transaction.

The service must not rely only on a UI button or route registration for authorization.

### 3. Make the Transaction Explicit

Inside one PostgreSQL transaction:

1. Insert company.
2. Insert the four system tenant roles.
3. Attach the default permission matrix.
4. Insert initial Super Admin.
5. Assign the `super_admin` role with `warehouse_id = NULL`.
6. Commit.

Use a unique constraint on `companies.slug` and map the constraint violation to `409`.

Add a transaction rollback test that forces a failure after role creation and verifies no partial company remains.

### 4. Default Permission Matrix

Seed the roles consistently:

- `super_admin`: admin management, role management, user creation, warehouse type management, warehouse management, membership management, warehouse viewing, worker task execution.
- `admin`: user creation, warehouse type management, warehouse management, membership management, warehouse viewing, worker task execution.
- `manager`: membership management, warehouse viewing, worker task execution.
- `worker`: warehouse viewing, worker task execution.

Do not copy permissions from the Product Owner role into tenant roles.

### 5. Return and Audit

Return only non-sensitive company and Super Admin fields. Add an audit event containing:

- Platform user ID
- Created company ID
- Initial Super Admin ID
- Request ID
- Timestamp

Do not include passwords or tokens in the audit payload.

## Web UI Work Plan

### Routes

Use the platform session namespace:

- `/platform/login`
- `/platform`
- `/platform/companies/new`

Keep the platform refresh cookie separate from the tenant refresh cookie.

### Platform Workspace

After platform session restoration:

- Call `/api/v1/platform/me` or use the login response.
- Show platform operator identity.
- Show a `Create company` action only when `companies.create` is present.
- Keep platform branding distinct from tenant warehouse work.
- Provide sign out that clears only platform session state.

### Company Form

Fields:

- Company name
- Company slug
- Super Admin name
- Super Admin email
- Password
- Confirm password

Client validation:

- Required fields.
- Slug format.
- Email format.
- Minimum password length.
- Password confirmation match.

The client sends only:

```json
{
  "name": "...",
  "slug": "...",
  "super_admin": {
    "name": "...",
    "email": "...",
    "password": "..."
  }
}
```

### Form States

Implement explicit states:

- Idle
- Submitting
- Validation error
- Duplicate slug
- Server error
- Success

During submission:

- Disable duplicate submits.
- Keep entered non-password fields.
- Clear password fields after success or failed submission.
- Show a generic error for unexpected failures.
- Move focus to the error message for accessibility.

### Success Screen

Display:

- Company name and slug
- Initial Super Admin name and email
- A clear next step: sign in at the tenant login screen

Do not display or retain the password.

## Test Plan

### Backend Unit Tests

- Valid onboarding input succeeds.
- Invalid slug fails.
- Short password fails.
- Missing Super Admin data fails.
- Product Owner permission is required.
- Non-Product Owner platform actor receives `403`.

### Backend Integration Tests

1. Bootstrap Product Owner.
2. Log in through `/platform/auth/login`.
3. Call `/platform/me`.
4. Create company through `/platform/companies`.
5. Verify all four tenant roles exist.
6. Verify default role permissions exist.
7. Verify initial Super Admin exists with an Argon2id hash.
8. Verify the Super Admin assignment is company-scoped.
9. Sign in through tenant `/auth/login`.
10. Verify the Product Owner has no tenant assignment.
11. Repeat the company slug and verify `409`.
12. Force a post-company failure and verify transaction rollback.
13. Send a tenant token to the platform endpoint and verify `401`.

### Browser Tests

- Platform login success.
- Platform login invalid credentials.
- Session restoration after reload.
- Company form validation.
- Successful company creation.
- Duplicate slug error.
- Password is never shown after success.
- Platform logout returns to `/platform/login`.
- Tenant login works for the newly created Super Admin.

## Optimization and Code Quality

The implementation should be optimized for clarity, database efficiency, and predictable runtime behavior:

- Keep company onboarding inside one short-lived transaction and avoid network calls or password hashing while holding database locks.
- Use one PostgreSQL transaction with batched or prepared permission inserts where practical instead of issuing unnecessary per-permission round trips.
- Add indexes for company slug, platform email, tenant email, role assignments, and active refresh-token lookups.
- Select only the columns needed by login, permission, and onboarding responses; never load password hashes into API response models.
- Load platform permissions once per authenticated request and use `perm_version` to support safe caching and invalidation later.
- Avoid duplicate token-generation, refresh, or `/me` requests in the web client; allow only one refresh retry for a failed request.
- Keep password hashing parameters strong but configurable and benchmark Argon2id work outside database transactions.
- Use typed request/response models and shared validation instead of repeated ad hoc parsing.
- Keep platform and tenant repositories, middleware, cookies, and token claims separate to reduce branching and accidental authorization leaks.
- Prefer small services with explicit dependencies, context propagation, and bounded request timeouts.
- Measure onboarding latency, login latency, database query count, and refresh failures in integration tests before adding caching or abstraction layers.
- Run Go formatting, static checks, backend tests, frontend type/build checks, and browser tests as part of the completion gate.

## Delivery Order

1. Add transaction and constraint-focused backend tests.
2. Harden duplicate-slug and duplicate-admin error mapping.
3. Add audit event and request-ID correlation.
4. Complete the onboarding UI route and form states.
5. Add browser coverage for the full Product Owner-to-Super-Admin journey.
6. Document the development seed and production Product Owner bootstrap procedure.
7. Add mobile platform onboarding only when platform operations are required on mobile.

## Definition of Done

- Product Owner signs in through the platform endpoint.
- Product Owner creates a company from the platform UI.
- Company and initial Super Admin are created atomically.
- Initial Super Admin can sign in through tenant login immediately.
- No credentials or tokens leak into responses, logs, or UI state.
- Unauthorized platform users cannot create companies.
- Tenant and platform sessions remain isolated.
- Backend integration and browser tests cover the complete workflow.
