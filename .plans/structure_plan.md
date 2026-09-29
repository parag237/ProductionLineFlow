# Multi-Warehouse Management System: Structure Plan (v2)

**Implementation status:** Phase 1 scaffold complete. The backend skeleton, config loader, RBAC checks, FSM foundation, and the web frontend shell are in place and verified to compile successfully.

**v1 stack:** Go + Gin (API-only, FSM-driven) · PostgreSQL · Redis · TypeScript monorepo (React web + Expo mobile) · Server-driven UI · Env-based JSON config · Docker Compose

**What changed from v1 of this document:** multi-role RBAC with a permission catalog in the DB; no minimum manager/worker per warehouse (old §4 deleted); email unique per company; set-password on user create (no invite tokens); domain lifecycle via `POST /{resource}/{id}/events`; refresh tokens in Postgres; `APP_ENV` + `CONFIG_DIR`; v1 stack only (scale tech moved to the "Later" appendix).

---

## Table of Contents

1. [Requirements](#1-requirements)
2. [RBAC Model](#2-rbac-model)
3. [Data Model](#3-data-model)
4. [Authentication](#4-authentication)
5. [Authorization at Runtime](#5-authorization-at-runtime)
6. [FSM Architecture](#6-fsm-architecture)
7. [Server-Driven UI Contract](#7-server-driven-ui-contract)
8. [API Surface](#8-api-surface)
9. [Configuration](#9-configuration)
10. [v1 Tech Stack](#10-v1-tech-stack)
11. [Project Structure](#11-project-structure)
12. [Build Order](#12-build-order)
13. [Appendix: Later (not in v1)](#13-appendix-later-not-in-v1)

---

## 1. Requirements

- A company has multiple warehouses. Each has its own name and its own type (factory, packing department, dispatch, or any other department). Types are data.
- One **Super Admin** per company. Under them, multiple **Admins**, managed by the Super Admin only.
- A user can hold **multiple roles** (e.g. `admin` company-wide, `manager` of WH-A, `worker` of WH-B).
- **Warehouses have no minimum staffing.** A warehouse may have zero managers, zero workers, or any mix.
- A Manager controls Workers in warehouses where they hold the right permission. An Admin controls everything except admin management and role permissions.
- Backend: **Go only**, API-based, **Gin**, **finite state machine** architecture.
- Frontend: **TypeScript monorepo**, separate from the backend, API-only interaction. It renders screens described by the backend.
- Config via local `config.json` files, environment-based (dev / test / prod).
- **Email is unique per company**, not globally. A user belongs to exactly one company.

---

## 2. RBAC Model

```mermaid
flowchart LR
  User --> Assignment
  Role --> Assignment
  Warehouse -.-> Assignment
  Permission --> RolePermission
  Role --> RolePermission
  Assignment --> CanCheck["Actor.Can(perm, warehouse)"]
```

### 2.1 Scope rules

- **Company-scoped roles** (`super_admin`, `admin`, later custom company roles): `warehouse_id` is NULL. Grants apply in **every** warehouse of that company.
- **Warehouse-scoped roles** (`manager`, `worker`, later custom warehouse roles): `warehouse_id` is required. Grants apply **only** in that warehouse.
- Effective access is the **union** of all of a user's assignments.
- Checks use **permissions**, never a single role string.

### 2.2 Permission keys (v1 seed)

Keys are migration-owned and stable. Super Admin can attach/detach them on **their own company's** roles, but cannot create new keys in v1. Custom **roles** are allowed once the UI exists; they only combine existing keys.

| Key | Meaning | Seeded on |
|---|---|---|
| `admins.manage` | Create/edit/delete admins, assign `admin`/`super_admin` | `super_admin` only |
| `roles.manage` | Edit `role_permissions`, create custom roles | `super_admin` only |
| `users.create` | Create company-level users | `super_admin`, `admin` |
| `warehouse_types.manage` | CRUD warehouse types | `super_admin`, `admin` |
| `warehouses.manage` | Create/edit/archive warehouses, fire domain events | `super_admin`, `admin` |
| `warehouse.members.manage` | Assign warehouse-scoped roles in a manageable warehouse | `super_admin`, `admin`, `manager` |
| `warehouse.view` | View a warehouse and its members | all four |
| `workers.tasks.execute` | Operational worker actions | all four |
| `stock.view` / `stock.adjust` | When inventory exists | later |

(Finalize the exact matrix during implementation.)

### 2.3 Who can assign what

| Actor | May assign / remove |
|---|---|
| **Super Admin** | Any role, including admins. Cannot remove the last `super_admin`. |
| **Admin** | Warehouse-scoped roles (`manager`, `worker`, custom warehouse roles) in any warehouse. Cannot edit `role_permissions`. Cannot assign `admin` / `super_admin`. |
| **Manager** | `worker` only (v1), in warehouses where they hold `warehouse.members.manage`. |
| **Worker** | Nothing. |

### 2.4 Guards on editing `role_permissions`

- System roles cannot be deleted and their slugs cannot change.
- Cannot strip `admins.manage` and `roles.manage` from `super_admin` (company lockout).
- Only holders of `roles.manage` mutate the catalog, and only for their own company.
- `admin` / `super_admin` assignment goes through Super Admin only, except a dedicated **transfer super admin** flow.
- **Last Super Admin protection** stays: the last active `super_admin` cannot be unassigned or deactivated. This is company lockout protection, not warehouse staffing.

---

## 3. Data Model

### 3.1 Tenancy and users

```sql
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE companies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL UNIQUE,            -- used at login
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id UUID NOT NULL REFERENCES companies(id),
  name TEXT NOT NULL,
  email CITEXT NOT NULL,
  password_hash TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  perm_version INT NOT NULL DEFAULT 1,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_company_email ON users (company_id, email);
```

There is no `users.role` column. Deactivated users still occupy their `(company_id, email)` slot; reuse means reactivating that row or changing the old email.

### 3.2 Warehouses

```sql
CREATE TABLE warehouse_types (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id UUID NOT NULL REFERENCES companies(id),
  name TEXT NOT NULL,                   -- "Factory", "Packing Department"
  UNIQUE (company_id, name)
);

CREATE TABLE warehouses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id UUID NOT NULL REFERENCES companies(id),
  type_id UUID NOT NULL REFERENCES warehouse_types(id),
  name TEXT NOT NULL,
  address TEXT,
  state TEXT NOT NULL DEFAULT 'draft',  -- domain FSM state
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (company_id, name)
);
```

There is no `warehouse_members` table. Assignments are membership.

### 3.3 RBAC tables

```sql
-- Global catalog (migrations seed keys)
CREATE TABLE permissions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key TEXT NOT NULL UNIQUE,             -- 'admins.manage'
  description TEXT NOT NULL
);

CREATE TABLE roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id UUID NOT NULL REFERENCES companies(id),
  slug TEXT NOT NULL,                   -- 'super_admin','admin','manager','worker', or custom
  name TEXT NOT NULL,
  scope TEXT NOT NULL CHECK (scope IN ('company','warehouse')),
  is_system BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (company_id, slug)
);

CREATE TABLE role_permissions (
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id UUID NOT NULL REFERENCES permissions(id),
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_role_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id UUID NOT NULL REFERENCES companies(id),
  user_id UUID NOT NULL REFERENCES users(id),
  role_id UUID NOT NULL REFERENCES roles(id),
  warehouse_id UUID REFERENCES warehouses(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per (user, role) when company-scoped; per (user, role, warehouse) when warehouse-scoped
CREATE UNIQUE INDEX ura_company_scoped
  ON user_role_assignments (user_id, role_id) WHERE warehouse_id IS NULL;
CREATE UNIQUE INDEX ura_warehouse_scoped
  ON user_role_assignments (user_id, role_id, warehouse_id) WHERE warehouse_id IS NOT NULL;
CREATE INDEX ON user_role_assignments (user_id);
CREATE INDEX ON user_role_assignments (warehouse_id);
```

**Assignment integrity (DB + service):**

- Row `company_id` must match `users.company_id`, `roles.company_id`, and the warehouse's `company_id` when set.
- `roles.scope = 'company'` ⇒ `warehouse_id IS NULL`. `roles.scope = 'warehouse'` ⇒ `warehouse_id IS NOT NULL`. A single-table CHECK cannot see `roles.scope`, so enforce it in the service inside the assignment transaction (or with a trigger).
- **At most one active Super Admin per company.** A partial unique index cannot join to `users.is_active`, so enforce it in the service: lock the company row (`SELECT ... FOR UPDATE`), then count active `super_admin` assignments before assigning or unassigning.
- **Seed the four system roles per company** when the company is created, with default `role_permissions`. Super Admin of company A changing Manager permissions never affects company B.

### 3.4 Flow sessions and refresh tokens

```sql
CREATE TABLE flow_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id UUID NOT NULL REFERENCES companies(id),
  user_id UUID NOT NULL REFERENCES users(id),
  flow_type TEXT NOT NULL,
  state TEXT NOT NULL,
  context JSONB NOT NULL DEFAULT '{}',
  version INT NOT NULL DEFAULT 1,
  completed BOOLEAN NOT NULL DEFAULT false,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON flow_sessions(user_id) WHERE NOT completed;

CREATE TABLE refresh_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id UUID NOT NULL REFERENCES companies(id),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  replaced_by UUID REFERENCES refresh_tokens(id),
  user_agent TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
```

### 3.5 Roadmap tables (not in the first schema)

`items`, `stock_balances`, `stock_movements` (append-only ledger), `outbox`, `audit_log`. Add these when inventory work begins. See the appendix.

---

## 4. Authentication

### 4.1 Login

Email alone cannot identify a user, since the same address can exist in two companies.

```
POST /api/v1/auth/login
{ "company_slug": "acme", "email": "a@acme.com", "password": "..." }
```

### 4.2 Create user: set password (no invite tokens)

```
POST /api/v1/users
{
  "name": "...",
  "email": "...",
  "password": "...",
  "assignments": [ { "role_id": "...", "warehouse_id": null } ]   // optional
}
```

- `password` is required, minimum length enforced server-side, hashed with **argon2id**, never stored or returned in plaintext.
- `company_id` comes from the JWT, never the body.
- Optional `assignments` run in the same transaction. Otherwise use `POST /users/{id}/role-assignments` afterwards.
- Who may create: `users.create` for company users, or `warehouse.members.manage` in the target warehouse (a Manager creating a worker).
- Password changes: `PATCH /me/password` `{ current_password, new_password }` for self-service. `PATCH /users/{id}/password` `{ "password": "..." }` for an admin reset (same permission as create). The last Super Admin can only be reset by themselves via `/me`.
- SDUI create-user screens show password + confirm fields. Confirmation is client-side only; the server accepts a single `password`.

### 4.3 Tokens

- Access JWT (15 min). Claims: `user_id`, `company_id`, `perm_version`. **No `role` claim.**
- Refresh tokens are opaque; only a **hash** is stored in `refresh_tokens`.
- Login inserts a row and returns access JWT + refresh token (TTL from config, e.g. 168h).
- `POST /auth/refresh`: look up by hash, reject if missing/expired/revoked. **Rotate**: revoke the old row, insert a new one, set `replaced_by`. Reuse of a revoked token that has `replaced_by` revokes the whole chain for that user (theft signal).
- `POST /auth/logout`: revoke the presented token (optionally all for the user).
- Deactivating a user revokes all outstanding rows for that `user_id`.
- A periodic job deletes expired rows.
- Web: refresh token in an httpOnly cookie. Mobile: secure store. Access token is a bearer token.

---

## 5. Authorization at Runtime

Each request loads the user's assignments plus permission keys from Postgres, cached in Redis by `user_id` + `perm_version`.

```go
func (a Actor) Can(perm string, warehouseID *uuid.UUID) bool {
    for _, as := range a.Assignments {
        if !as.Has(perm) { continue }
        if as.WarehouseID == nil { return true }                                 // company-scoped
        if warehouseID != nil && *as.WarehouseID == *warehouseID { return true } // warehouse-scoped
    }
    return false
}
```

- **Middleware:** require auth and take the tenant from the token. It does **not** gate routes on a single role.
- **Service / FSM:** call `Can(permission, warehouseID)` on every mutating event.
- **Navigation:** the union of screens any assignment allows. The warehouse switcher lists warehouses the user can `warehouse.view`.
- **UI hiding is never authorization.**
- `GET /me` returns assignments (`role` slug, scope, `warehouse_id`) and the effective permission map (company-wide keys plus per-warehouse keys).

### Cache invalidation

When an assignment or `role_permissions` row changes, bump `users.perm_version` (for a role's permission-set change, bump every user holding that role). A client with a stale JWT `perm_version` refetches `/me` or receives `401` with `code=permissions_changed`.

---

## 6. FSM Architecture

### 6.1 Two kinds of state machines

| Type | Controls | HTTP shape |
|---|---|---|
| **Flow FSM** | A user's journey through screens (wizards, approvals) | `POST /flows/:type`, `GET /flows/:id`, `POST /flows/:id/events` → returns a **screen** |
| **Domain FSM** | Lifecycle of a business entity | `GET /{resource}/:id`, `POST /{resource}/:id/events` → returns the **resource** + `state` + `allowed_events` |

Do **not** `PATCH` an entity's `state`. Lifecycle changes go through events only. Lists and dashboards are plain `GET /screens/:name` with no session. A state machine for a table view adds latency for no gain.

### 6.2 Engine

A small in-house engine (not an in-memory library) because persistence, permission guards, transactional actions and state-to-screen mapping are required. Transitions carry a **Permission**, not a role list.

```go
package fsm

type State string
type Event string

type Transition struct {
    From       State
    Event      Event
    To         State
    Permission string // e.g. "warehouses.manage"; checked with Actor.Can(perm, scopeWarehouseID)
    Guard      func(ctx context.Context, tx Tx, s *Session, p Payload) error
    Action     func(ctx context.Context, tx Tx, s *Session, p Payload) error // inside the DB tx
}

type ScreenBuilder func(ctx context.Context, s *Session, a Actor) ui.Screen

type Definition struct {
    Type        string
    Initial     State
    Terminal    map[State]bool
    Transitions []Transition
    Screens     map[State]ScreenBuilder // flow FSMs only
}
```

```go
func (e *Engine) Fire(ctx context.Context, a Actor, flowID uuid.UUID,
    ev Event, p Payload) (*Result, error) {

    var res *Result
    err := e.store.WithTx(ctx, func(tx Tx) error {
        s, err := tx.LoadForUpdate(ctx, flowID, a.CompanyID, a.ID) // SELECT ... FOR UPDATE
        if err != nil { return err }

        def := e.defs[s.Type]
        t, ok := def.Find(s.State, ev)
        if !ok {
            return &InvalidEventError{State: s.State, Allowed: def.Allowed(s.State, a)} // 409
        }
        if !a.Can(t.Permission, s.WarehouseID()) { return ErrForbidden }               // 403
        if t.Guard != nil  { if err := t.Guard(ctx, tx, s, p);  err != nil { return err } }
        if t.Action != nil { if err := t.Action(ctx, tx, s, p); err != nil { return err } }

        s.Context.Merge(p)
        s.State = t.To
        s.Version++
        if def.Terminal[s.State] { s.Completed = true }
        if err := tx.Save(ctx, s); err != nil { return err }

        res = &Result{Session: s, Screen: def.Screens[s.State](ctx, s, a)}
        return nil
    })
    return res, err
}
```

The domain FSM uses the same engine core, with the entity row (e.g. `warehouses`) locked `FOR UPDATE` instead of a `flow_sessions` row.

**Safety:** `FOR UPDATE` serializes double-taps and concurrent tabs. An `Idempotency-Key` header (stored in Redis, which is a day-one service) makes a retry return the stored result instead of firing the transition twice. Purge expired flow sessions periodically.

### 6.3 Flow: create warehouse

The wizard collects warehouse details and type only. People are assigned later through `user_role_assignments`.

```go
var WarehouseCreate = fsm.Definition{
    Type:    "warehouse_create",
    Initial: "details",
    Terminal: map[fsm.State]bool{"done": true, "cancelled": true},
    Transitions: []fsm.Transition{
        {From: "details", Event: "SUBMIT_DETAILS", To: "review",    Permission: "warehouses.manage", Guard: validateDetails},
        {From: "review",  Event: "BACK",           To: "details",   Permission: "warehouses.manage"},
        {From: "review",  Event: "CONFIRM",        To: "done",      Permission: "warehouses.manage", Action: createWarehouse},
        {From: "details", Event: "CANCEL",         To: "cancelled", Permission: "warehouses.manage"},
    },
    Screens: map[fsm.State]fsm.ScreenBuilder{
        "details": screens.WarehouseDetails,
        "review":  screens.WarehouseReview,
        "done":    screens.WarehouseCreated,
    },
}
```

### 6.4 Domain FSM: warehouse lifecycle

```
GET  /api/v1/warehouses/{id}          -> warehouse + state + allowed_events (permission-filtered)
POST /api/v1/warehouses/{id}/events   -> header Idempotency-Key; body { "event": "ACTIVATE", "payload": {} }
```

The engine loads the warehouse `FOR UPDATE`, checks `Can("warehouses.manage", warehouseID)`, runs guard and action, and writes the new state. The response is the updated warehouse with `state` and `allowed_events`, not a screen.

| From | Event | To |
|---|---|---|
| `draft` | `ACTIVATE` | `active` |
| `active` | `SUSPEND` | `suspended` |
| `suspended` | `REACTIVATE` | `active` |
| `active` / `suspended` | `ARCHIVE` | `archived` |

**No staffing guards.** An empty warehouse can be `active`. When inventory exists, `ARCHIVE` gains a "no pending stock / open transfers" guard. In v1 without inventory, archiving is unguarded.

`PATCH /warehouses/{id}` edits name, address and type only, and **rejects** `state` in the body. The same pattern applies to later entities, e.g. `POST /transfers/{id}/events`.

---

## 7. Server-Driven UI Contract

The frontend is a **renderer**: fetch a screen, draw it, send events back.

### 7.1 Response shape

```json
{
  "flow_id": "b3f1...",
  "state": "details",
  "allowed_events": ["SUBMIT_DETAILS", "CANCEL"],
  "screen": {
    "name": "warehouse.create.details",
    "schema_version": 1,
    "title": "Warehouse details",
    "components": [
      { "type": "form", "id": "details_form", "fields": [
        { "type": "text_input", "name": "name", "label": "Warehouse name",
          "rules": { "required": true, "min": 2 } },
        { "type": "select", "name": "type_id", "label": "Type",
          "options_source": "warehouse_types", "rules": { "required": true } }
      ]}
    ],
    "actions": [
      { "id": "cancel", "label": "Cancel", "event": "CANCEL", "style": "secondary" },
      { "id": "next", "label": "Next", "event": "SUBMIT_DETAILS",
        "style": "primary", "submits": "details_form" }
    ]
  }
}
```

### 7.2 Rules

1. **Fixed component vocabulary:** `heading`, `text`, `form`, `text_input`, `password_input`, `select`, `table`, `card`, `stat`, `list`, `banner`, `button`, `tabs`, `scanner` (mobile only). The backend composes only these.
2. **`schema_version` on every screen.** Unknown component types render a fallback ("Update the app to see this screen") instead of crashing.
3. **The server is the authority.** Field `rules` are client hints; the server re-validates every event.
4. **Localized strings come from the server** (`Accept-Language`).
5. **Keep high-frequency screens native** (barcode scanning, offline task entry). Server-driven screens can be cached with `ETag` for offline reading. Offline writes use a client-side queue with idempotency keys.
6. **Type sync:** the source of truth for SDUI types is an explicit OpenAPI spec / JSON Schema generated from the Go `ui` types (not `swag` annotations). Generate the TS types from it in CI so a Go component change breaks the TS build, not production.

### 7.3 Client loop

`GET /flows/:id` → `<ScreenRenderer />` looks up each `type` in the platform registry → a button press sends `POST /flows/:id/events` with the form payload → render the next screen. Web and mobile share `sdui-core`; only the registries differ.

---

## 8. API Surface

All routes under `/api/v1`. Additive changes only, since mobile apps lag.

```
# Auth
POST  /auth/login | /auth/refresh | /auth/logout
GET   /me
PATCH /me/password

# Server-driven UI
GET   /navigation                        # derived from the permission union
GET   /screens/:name                     # stateless screens (dashboards, lists)
POST  /flows/:type                       # start flow -> first screen
GET   /flows/:id                         # resume -> current screen
POST  /flows/:id/events                  # Idempotency-Key

# Users and RBAC
GET/POST   /users                        # POST requires password
PATCH      /users/{id}
PATCH      /users/{id}/password
POST       /users/{id}/role-assignments  # { role_id, warehouse_id? }
DELETE     /users/{id}/role-assignments/{assignmentId}
GET/POST   /roles                        # roles.manage
PUT        /roles/{id}/permissions       # roles.manage

# Warehouses (domain FSM)
GET/POST          /warehouse-types
GET/POST          /warehouses
GET/PATCH         /warehouses/{id}       # PATCH rejects `state`
POST              /warehouses/{id}/events  # Idempotency-Key
GET               /warehouses/{id}/members
```

`POST /warehouses/{id}/managers` and the earlier `/workers` routes are replaced by `POST /users/{id}/role-assignments`.

### Gin router skeleton

```go
func NewRouter(cfg *config.Config, d *Deps) *gin.Engine {
    if cfg.Env == "prod" { gin.SetMode(gin.ReleaseMode) }
    r := gin.New()
    r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(d.Log),
          middleware.CORS(cfg.Server.AllowedOrigins))

    v1 := r.Group("/api/v1")
    v1.POST("/auth/login", d.Auth.Login)
    v1.POST("/auth/refresh", d.Auth.Refresh)

    sec := v1.Group("", middleware.JWT(cfg.Auth), middleware.LoadActor(d.Perms)) // no role gate
    sec.GET("/me", d.Me.Get)
    sec.GET("/navigation", d.Nav.Get)
    sec.GET("/screens/:name", d.Screens.Get)
    sec.POST("/flows/:type", d.Flows.Start)
    sec.GET("/flows/:id", d.Flows.Get)
    sec.POST("/flows/:id/events", middleware.Idempotency(d.Redis), d.Flows.Fire)

    sec.GET("/warehouses/:id", d.Warehouses.Get)
    sec.PATCH("/warehouses/:id", d.Warehouses.Update)
    sec.POST("/warehouses/:id/events", middleware.Idempotency(d.Redis), d.Warehouses.Fire)
    return r
}
```

### API rules

- Rate limit per user and per company (Redis token bucket).
- Keyset (cursor) pagination on list endpoints. No `OFFSET` on large tables.
- Every query filters by `company_id` from the token.
- Timeouts and context propagation on every outbound call.

---

## 9. Configuration

Two documented environment variables:

- `APP_ENV`: `dev` | `test` | `prod`, default `dev`.
- `CONFIG_DIR`: directory of JSON files, default `config` if unset. The loader always reads the variable; Compose and prod always set it.

```
warehouse-api/config/
  config.dev.json
  config.test.json
  config.prod.json
  config.example.json      # committed template, no real secrets
```

### Example `config.dev.json`

```json
{
  "env": "dev",
  "server": { "port": 8080, "read_timeout_sec": 15, "write_timeout_sec": 30,
              "allowed_origins": ["http://localhost:5173"] },
  "database": { "dsn": "postgres://app:app@localhost:5432/warehouse_dev?sslmode=disable",
                "max_conns": 10 },
  "redis": { "addr": "localhost:6379", "db": 0 },
  "auth": { "jwt_secret": "dev-only-secret", "access_ttl_min": 15, "refresh_ttl_hours": 168,
            "min_password_length": 10 },
  "flows": { "session_ttl_min": 60 },
  "log": { "level": "debug", "format": "text" }
}
```

### Loader

```go
package config

func Load() (*Config, error) {
    env := os.Getenv("APP_ENV")
    if env == "" { env = "dev" }
    if env != "dev" && env != "test" && env != "prod" {
        return nil, fmt.Errorf("invalid APP_ENV %q (want dev|test|prod)", env)
    }

    dir := os.Getenv("CONFIG_DIR")
    if dir == "" { dir = "config" }
    f, err := os.Open(filepath.Join(dir, "config."+env+".json"))
    if err != nil { return nil, fmt.Errorf("open config: %w", err) }
    defer f.Close()

    var c Config
    dec := json.NewDecoder(f)
    dec.DisallowUnknownFields()
    if err := dec.Decode(&c); err != nil { return nil, fmt.Errorf("parse config: %w", err) }

    if c.Env != env { return nil, fmt.Errorf("config env %q does not match APP_ENV %q", c.Env, env) }
    return &c, c.Validate() // prod rejects the dev JWT secret and short secrets
}
```

- Add `config.prod.json` and `config.test.json` to `.gitignore` if they hold real secrets; commit only `config.example.json`.
- Tests set `CONFIG_DIR` to a fixture path. The DSN stays inside that JSON unless overlays are added later.
- Frontend config follows the same pattern (`apps/web/config/config.{env}.json`, `apps/mobile/config/...` read in `app.config.ts`) and is **public**: no secrets.

---

## 10. v1 Tech Stack

| Concern | Choice |
|---|---|
| Backend | Go, Gin |
| DB access / migrations | `pgx` + `sqlc`, `golang-migrate` |
| Auth | `golang-jwt/jwt`, argon2id |
| Validation | `go-playground/validator` |
| Logging | `log/slog` |
| Database | PostgreSQL (single primary; replica optional later) |
| Cache / rate limit / idempotency | Redis |
| Testing | `testing`, `testify`, `testcontainers-go` |
| Deploy (now) | Docker Compose: API + Postgres + Redis (not Kubernetes) |
| Frontend monorepo | Turborepo + pnpm |
| Web | React + Vite, Tailwind + shadcn/ui, TanStack Table |
| Mobile | Expo (React Native), NativeWind |
| Client data / forms | TanStack Query, React Hook Form + Zod |
| API types | OpenAPI / JSON Schema from Go `ui` types → `openapi-typescript` |
| Auth storage | Web: memory + httpOnly cookie. Mobile: `expo-secure-store` |

**Architecture:** modular monolith with strict package boundaries (no cross-package DB access), so a module can be extracted later.

**UI language:** TypeScript across web and mobile. About 60–70% of the code (API client, schemas, permissions, SDUI core, data hooks) is shared; screens render per platform through separate component registries.

---

## 11. Project Structure

### Backend: `warehouse-api/`

```
cmd/api/main.go
config/                     # config.{dev,test,prod}.json + example
internal/
  config/                   # loader + validation
  fsm/                      # engine, session store, errors
  flows/                    # warehouse_create.go (flow definitions)
  domain/                   # warehouse_lifecycle.go (domain FSM definitions)
  ui/                       # Screen, Component, Action types + builders
  screens/                  # stateless screen builders + registry
  navigation/               # permission-derived menu
  auth/                     # login, refresh rotation, JWT, argon2id
  rbac/                     # Actor, Can(), assignments, role/permission admin
  user/  warehouse/
  platform/{db,httpx,cache}/
db/
  migrations/
  queries/                  # sqlc
sqlc.yaml
Dockerfile
docker-compose.yml
```

Flow: **handler → service/FSM → repository**. Business rules live only in the service and FSM layers.

### Frontend: `warehouse-frontend/`

```
apps/
  web/            # React + Vite shell, layout, table-heavy screens
  mobile/         # Expo shell + native scanner/offline screens
packages/
  sdui-core/      # Screen/Component TS types, flow + domain-event client, useFlow()
  sdui-web/       # component registry -> shadcn/Tailwind
  sdui-mobile/    # component registry -> React Native
  api-client/     # generated from the OpenAPI spec
  schemas/        # shared Zod validation
  permissions/    # can(perm, warehouseId) helper for UI hints
  ui-tokens/
```

---

## 12. Build Order

1. Config loader (`APP_ENV` + `CONFIG_DIR`), Gin skeleton, Docker Compose, migrations (companies, users, RBAC tables, `flow_sessions`, `refresh_tokens`).
2. Company bootstrap command: create company, seed four system roles with default permissions, create the Super Admin (with password).
3. Auth: login with company slug, refresh rotation, logout, `/me`, `Actor.Can`, `perm_version` cache.
4. FSM engine with unit tests. Transition tables are pure logic and easy to test.
5. Users and role assignments APIs with the assignment integrity rules and last-Super-Admin protection.
6. `ui` package and the `warehouse_create` flow end to end, plus the warehouse domain FSM (`POST /warehouses/{id}/events`).
7. `sdui-core` + web registry rendering those flows; `/navigation` from the permission union.
8. Role/permission admin screens (`roles.manage`), then mobile registry.
9. Inventory ledger and native scanner screens when inventory work starts.

No application code is written until the repo build is explicitly requested.

---

## 13. Appendix: Later (not in v1)

Introduce these only when load or requirements demand them. Nothing here is a current choice.

- **PgBouncer** for connection pooling; Postgres read replicas.
- **Inventory data model:** append-only `stock_movements` (monthly partitions, unique idempotency key), `stock_balances` updated in the same transaction, keyset pagination, hot-row mitigation.
- **Transactional outbox + Kafka/Redpanda/NATS** for event streaming.
- **ClickHouse** for analytics; **OpenSearch/Meilisearch** for search.
- **S3-compatible object storage** for documents and exports (asynchronous `202 Accepted` jobs).
- **Kubernetes with HPA, Argo CD**, API gateway, OpenTelemetry/Prometheus/Grafana stack.
- **Service extraction** (inventory first) and tenant sharding (`company_id` is already on every table).
- **Email-only login** with a company picker, invite/reset-by-email flows.

| Stage | Scale | Setup |
|---|---|---|
| 1 (v1) | Up to ~1M tx/day | Modular monolith, Postgres, Redis, Compose |
| 2 | ~1M–50M/day | Partitioning, PgBouncer, outbox + Kafka, ClickHouse reports, autoscaled pods |
| 3 | 50M+/day | Extracted inventory service, sharding, multi-region reads |