# Company People and Roles Management

## Purpose and scope

Build a tenant scoped People & roles workspace in the company dashboard. The People card opens the workspace using the same in page navigation and return to overview pattern as Warehouses. This is a web feature; platform user management and invitation workflows are out of scope.

## Existing foundations

Use the existing `users`, `roles`, `permissions`, `role_permissions`, and `user_role_assignments` tables. Company ID comes only from the authenticated tenant actor. Preserve existing database assignment triggers, company boundaries, permission aggregation, and token version checks. Use current schema unless implementation proves an additional field or constraint is necessary.

## Workspace and people directory

The People & roles card is visible to company users with `users.create` and managers with `warehouse.members.manage` in at least one warehouse. Provide searchable people rows, active/inactive filters, names, company email, role assignments, warehouse names, and clear loading, empty, validation, conflict, and server error states.

Company Super Admins and authorized Admins can create, view, edit, deactivate, and reactivate people. Creation requires name, company email, password, and optional role assignment(s). Editing changes shared identity fields. Deactivation is soft deletion: preserve user, email, and assignments; increment permission version and revoke refresh tokens. Reactivation keeps existing assignments. Password reset is a separate operation and must never return password material. Users can change their own password after current password verification.

Managers can list and view only worker accounts assigned in warehouses where they have `warehouse.members.manage`. They can add or remove Worker assignments in those warehouses. They cannot edit shared identity, reset passwords, deactivate accounts, change non Worker assignments, or access people outside their scope. Workers receive no People navigation or API access.

## API surface

All routes require tenant authentication and derive company scope from the actor:

- `GET /users?q=&status=`, `GET /users/{id}`
- `POST /users`, `PATCH /users/{id}`, `DELETE /users/{id}` (deactivate; PATCH status reactivates)
- `PATCH /users/{id}/password`, `PATCH /me/password`
- `POST /users/{id}/role-assignments`, `DELETE /users/{id}/role-assignments/{assignmentId}`
- `POST /users/{id}/transfer-super-admin`
- `GET /roles`, `POST /roles`, `PATCH /roles/{id}`, `DELETE /roles/{id}`
- `GET /permissions` for the authorized custom-role permission editor.

Never accept `company_id` in request bodies. Use Argon2id password hashes and return safe person DTOs only. Enforce company email uniqueness. Translate duplicate email, invalid scope, and assigned role deletion into useful client conflicts.

## Role and assignment rules

Allow multiple assignments per person. Company scoped Admin access applies company wide. Warehouse scoped Manager and Worker roles require a warehouse owned by the same tenant. Admins may manage non Admin people and warehouse scoped memberships, but cannot manage Admins, Super Admins, or role definitions. Super Admins manage all tenant people and custom roles.

Role checks are permission based. Managers use `warehouse.members.manage` plus the assigned warehouse ID and may only manage Worker membership there. Do not depend on role names for authorization except enforcing the protected Super Admin identity and the explicitly defined Worker membership capability.

Custom role CRUD is Super Admin only. Custom roles may combine known catalog permissions except `admins.manage` and `roles.manage`, reserved to system Super Admin. System roles cannot be changed or deleted. A custom role assigned to any user cannot be deleted; return conflict. Role permission changes increment `perm_version` for every affected user.

Ordinary create, edit, and assignment endpoints cannot grant or remove Super Admin. Keep exactly one active Super Admin per company. The dedicated transfer operation runs atomically: assign Super Admin to an active target and demote the outgoing Super Admin to Admin. Reject self transfer, inactive targets, and any operation that leaves no active Super Admin.

## Data integrity and security

Keep creates and transfers transactional, including assignments and permission versions. Validate every user, role, and warehouse against the actor's company. Deactivation revokes refresh tokens and invalidates access through `perm_version`. Password changes also revoke refresh tokens. Do not leak cross tenant existence through queries. Keep inactive users' email reserved.

## Validation and acceptance

Unit/service coverage: input validation, password hashing, company email conflicts, scope validation, authorization boundaries for Super Admin/Admin/Manager/Worker, reactivation and deactivation, role deletion conflict, protected permissions, and transfer restrictions.

Database/API integration coverage: tenant isolation, assignment integrity, permission version updates, refresh token revocation, and rollback when a multi step create or transfer fails.

Web acceptance: people CRUD, reset password, active/inactive filters, multi warehouse assignments, custom role CRUD and permission selection, manager warehouse limits, unauthorized access, and clear loading/empty/validation/conflict/server error states.

Acceptance scenario: a company Super Admin creates an Admin and Worker, assigns Manager and Worker access across warehouses, edits and deactivates a person, reactivates them, and safely transfers Super Admin. A Manager can manage Worker membership only in authorized warehouses and cannot change shared identity or status.

