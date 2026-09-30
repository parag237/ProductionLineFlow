# Sliding Idle Timeout for Authenticated Sessions

## Goal

Keep tenant and platform sessions alive while the user continues making authenticated API requests. End the session after the configured idle period with no authenticated API activity, then return the browser to the appropriate login page.

This plan does not change the 15 minute access JWT into a long lived token. Access tokens remain short lived and are renewed through the existing refresh flow. Session revocation for logout, account deactivation, company suspension, permission changes, and refresh token reuse remains effective.

## Current behavior and relevant settings

- `auth.access_ttl_min` is currently 15 minutes. It controls the lifetime of a signed access JWT.
- `auth.refresh_ttl_hours` is currently 168 hours. Login creates a refresh token with that expiry; refresh rotates the token and gives its replacement another 168 hours.
- The web request helper attempts refresh after a protected API call receives 401. As a result, activity advances the refresh deadline when refresh happens, rather than extending an idle deadline on every authenticated API request.
- Refresh cookies currently use the `/api/v1/auth` path, so normal API handlers cannot see the refresh cookie.
- `flows.session_ttl_min` controls workflow records and is unrelated to authentication sessions. It must remain independent.

## Recommended design

Represent each login as a server side session with a stable session ID (`sid`). Put that ID in both the access JWT and the session's refresh token record. Keep tenant and platform sessions separate so their credentials and revocation rules cannot cross boundaries.

Add session records for tenant and platform logins. Each record stores its owner, `last_activity_at`, `idle_expires_at`, `revoked_at`, and creation time. The configured idle timeout is the duration added to the current time after authenticated activity. Do not add an absolute maximum lifetime in this change: the requested policy is inactivity based. Explicit revocation events still end a session.

On every successfully authenticated, protected API request:

1. Validate the access JWT and its `sid`.
2. Confirm the session belongs to the JWT's user and company (or platform user), is not revoked, and has not reached `idle_expires_at`.
3. Extend `last_activity_at` and `idle_expires_at` by the configured idle timeout.
4. Extend the current refresh token's database expiry and renew the browser cookie's Max-Age using the same deadline.
5. Continue through existing account-active, company-active, permission-version, and handler authorization checks.

An expired session must not be revived by a late API call or refresh. The refresh handler checks the server side session deadline before rotating its refresh token. Successful refresh extends the same session, not a new session.

### Cookie renewal

The existing refresh cookie is scoped only to the authentication routes, so normal API requests cannot renew its browser expiry. Scope the HttpOnly refresh cookie to `/api/v1` and, after a protected request is authenticated, reissue the same current cookie with Max-Age equal to the remaining idle timeout. The middleware must bind the cookie's hash to the active refresh token and the JWT's session ID before renewing it.

Keep the cookie HttpOnly, Secure in production, and SameSite protected. Do not expose the raw refresh token in JSON, JavaScript, or logs. Preserve tenant and platform cookie names and their separate API auth paths. Review CORS and CSRF behavior as part of this change because the browser will send the cookie on more API paths; protected write routes must continue to require a valid bearer access token.

### Idle UX

Have the backend return the effective idle timeout (or a session expiry timestamp) from login and refresh. The web client tracks an idle deadline and resets its timer after an authenticated API request. When that timer expires, clear the relevant in-memory access token and replace the current location with `/login` or `/platform/login`.

The server remains authoritative: every protected request and refresh checks `idle_expires_at`. A browser timer only makes an idle user see the login page at the expected time; it must not extend server state by itself. Handle multiple tabs by broadcasting activity/deadline updates or by querying the server deadline, so activity in one tab does not log another active tab out prematurely.

Login and logout requests do not count as activity that extends a session. An authenticated request counts even when the handler returns a business validation error, provided authentication and session checks succeeded. Unauthorized requests must never extend the deadline.

## Configuration and migration

- Keep `auth.access_ttl_min` as the short access-token lifetime.
- Define one explicit auth idle timeout. Either rename `refresh_ttl_hours` to `idle_timeout_hours` with a config compatibility period, or retain the existing key but document and expose it as the rolling inactivity window. Prefer the clearer `idle_timeout_hours` name.
- Do not reuse `flows.session_ttl_min`.
- Add a migration for tenant and platform session tables and link refresh-token rows to a session.
- Add `sid` to access JWT claims.
- Plan a deployment cutover for previously issued access and refresh tokens, which have no session ID. Safest default: revoke legacy refresh tokens and require one sign in after deployment. If zero forced sign-ins are required, define and test an explicit backfill/compatibility strategy before implementation.

## Implementation phases

1. **Auth model and config:** Define idle timeout configuration and tenant/platform session domain types; add session ID to access token claims.
2. **Persistence:** Add migration and repository methods to create, validate, touch, revoke, and rotate refresh tokens within a session. Keep session and refresh token updates transactional.
3. **Login, refresh, logout:** Create one session on login; preserve it during refresh rotation; revoke it on logout and existing security revocation paths. Ensure permission changes and account/company lifecycle checks still invalidate sessions as intended.
4. **Tenant and platform middleware:** Check session state on each protected request; touch idle expiry only after token/session validation; renew the cookie using the same idle deadline. Ensure both auth boundaries apply the same policy independently.
5. **Web client:** Consume the backend timeout/deadline, reset the local timer after authenticated API activity, synchronize tabs, and route idle expiry to the correct login page. Preserve the existing access-token refresh behavior.
6. **Observability and operations:** Log session IDs only in non-sensitive form; never log credentials or raw refresh tokens. Document idle timeout config, cookie scope, migration cutover, and local development behavior.

## Tests and acceptance

### Unit and service tests

- Login creates a session with expected initial deadline and a refresh token bound to its session.
- Refresh preserves the session ID and advances the idle deadline.
- Touching a valid session advances activity and expiry; expired or revoked sessions cannot be touched or refreshed.
- Tenant tokens cannot access platform sessions and platform tokens cannot access tenant sessions.
- Logout, user deactivation, company suspension, permission-version changes, and refresh reuse continue to invalidate access.
- Authenticated API validation errors count as activity; unauthorized and login requests do not.
- Cookie Max-Age follows the remaining server deadline and uses the correct tenant/platform cookie.

### Database/API integration tests

- A protected tenant request touches only the matching tenant session and its active refresh token.
- A protected platform request touches only its matching platform session.
- Idle deadline expiry returns 401 without reviving the session; the next client action is redirected to the correct login route.
- Concurrent authenticated requests and refresh rotation do not spuriously trigger refresh-token reuse revocation.
- Database errors during touch fail closed; they must not allow an expired session to continue.
- Deactivation and suspension revoke or invalidate all relevant sessions.

### Acceptance scenarios

- A user making API requests more frequently than the idle timeout remains signed in indefinitely while active.
- A user who stops making API requests is redirected to login when the configured idle window expires.
- An access JWT can expire during active use; the web app refreshes it and continues without interrupting the user.
- Activity in one open tab keeps the shared session active in another tab.
- Tenant and platform sessions expire and redirect independently.
- Logout, disabled accounts, suspended companies, and revoked sessions still end access immediately.

## Decisions to confirm during evaluation

- The actual inactivity window to use. Current refresh setting is seven days; keep that value unless the product intends a shorter idle timeout.
- Whether a one-time sign-in after deployment is acceptable for users with legacy tokens.
- Whether a browser restart should preserve the login. The recommended rolling persistent cookie supports it while active; the database remains the authority for idle expiry.

