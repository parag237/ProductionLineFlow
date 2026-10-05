## Plan: Standalone Warehouse Work Log

Build an independent operations work-log dashboard for recording work by date, warehouse, item, step, quantity, and performer. Add explicit read/create permissions and warehouse authorization. Keep all production-run functionality separate and unchanged.

**Steps**
1. [ ] Add authorization capabilities `operations.logs.view` and `operations.logs.create` to the permission catalog and default role grants. Grant both to admins and managers; admins operate company-wide, while managers are constrained to their assigned warehouses. Do not grant workers by default. Update Operations portal visibility to appear for users with either permission, while enforcing read and create separately.
2. [ ] Add the next database migration for a standalone work-log table. Store company, required warehouse, calendar work date, item, step, positive quantity, selected performer, recording user, and creation timestamp. Enforce same-company references and prevent invalid cross-warehouse/tenant data. Preserve historical item/step labels (and durable identifiers where possible), since item edits currently replace step rows; history must not disappear or silently change when the catalog changes. No reference to production runs or production steps.
3. [ ] Implement a dedicated backend work-log package and routes, separate from item production routes: date/warehouse-filtered list, create, and a create-authorized options endpoint for available items/steps and active performers in the selected warehouse. Validate permission scope, warehouse membership, item-step pairing, performer eligibility, date format, and quantity. Use the authenticated actor as `recorded_by`; `performed_by` is selected in the form. Keep company admins able to act across warehouses; limit managers to assigned warehouses.
4. [ ] Connect the Operations card in `Workspace.tsx` to a real dashboard through `App.tsx` state/view wiring. Build a responsive dashboard with a prominent “Log Today's Entry” form column: default the date to today (editable), require warehouse/item/step/quantity/performer, and refresh dependent choices when warehouse or item changes. Show a date-filtered work-log table alongside it for users with view permission; hide list/form portions independently according to the two permissions. Show clear loading, empty, validation, and submit states.
5. [ ] Add backend migration, service, route, permission, and warehouse/tenant-isolation tests; add frontend checks for permission-gated controls and entry submission/list refresh using existing project conventions. Do not add production coupling or modify production-run behavior.

**Relevant files**
- `productionlineflow-frontend/apps/web/src/App.tsx` — add Operations view state, dashboard rendering, and portal callback.
- `productionlineflow-frontend/apps/web/src/features/workspace/Workspace.tsx` — replace the current `workers.tasks.execute` gate with the new view/create visibility and wire the Operations card.
- `productionlineflow-frontend/apps/web/src/types/index.ts` — add the work-log and options response types if shared local types are needed.
- `productionlineflow-frontend/apps/web/src/features/operations/OperationsDashboard.tsx` — new isolated dashboard/form/list feature.
- `productionlineflow-api/internal/server/router.go` — register dedicated work-log endpoints independently from `registerItemRoutes`.
- `productionlineflow-api/internal/company/postgres_repository.go` and the permission catalog/role initialization path — register the new capabilities and grants.
- `productionlineflow-api/internal/rbac/actor.go`, `internal/server/middleware.go`, and existing service patterns — reuse actor permission and warehouse-scope checks.
- `productionlineflow-api/internal/items/postgres_repository.go` and `internal/people/postgres_repository.go` — reference catalog and active-member/warehouse relationships for option validation; avoid modifying production repository paths.
- `productionlineflow-api/db/migrations/` — add the next numbered migration for work-log storage and permission catalog data.
- `productionlineflow-api/internal/server/router_test.go`, plus new work-log service/repository tests following `internal/items/service_test.go` and `internal/people/service_test.go` — cover route and domain behavior.

**Verification**
1. [ ] Run backend tests from `productionlineflow-api`: `go test ./...`.
2. [ ] Run the web package build from `productionlineflow-frontend`: `pnpm --filter @warehouse/web build` (and the existing frontend test command if one is available when implementing).
3. [ ] Apply migrations to a test database and verify tenant/warehouse constraints, positive quantity, and historical entries surviving item-step edits.
4. [ ] Manually verify manager access is limited to assigned warehouses, admins can select any company warehouse, view-only users cannot create, create-only users cannot read the log list, and the Operations card appears only with a relevant permission.
5. [ ] Confirm no work-log code imports or reads production-run/production-step records and production endpoints/behavior remain unchanged.

**Decisions**
- “By whom” means a selectable performer; the authenticated actor is separately retained as the recorder.
- Work logs are warehouse-specific; warehouse is a required form field.
- Permissions are separate for viewing and creating. Defaults go to admins and managers, not workers; admin permissions are company-scoped and manager permissions are warehouse-scoped.
- The Operations portal currently has no functional dashboard: its card is gated by `workers.tasks.execute` and has no click handler. The implementation adds a dedicated dashboard.
- Existing item/step contracts do not provide a work-log record; the new storage/API is independent. Do not reuse or connect production tracking.
- Use date-only semantics for the work date; no timezone conversion should shift the selected calendar date.
