import { useEffect, useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import type { ApiError, Assignment, Permission, Person, Permissions, Role, User, Warehouse } from '../../types';

export default function PeopleDashboard({ user, permissions, onBack }: { user: User; permissions: Permissions; onBack: () => void }) {
  const canManageCompany = permissions.company.includes('users.create');
  const canManageAdmins = permissions.company.includes('admins.manage');
  const canManageRoles = permissions.company.includes('roles.manage');
  const managedWarehouseIds = Object.entries(permissions.warehouses).filter(([, perms]) => perms.includes('warehouse.members.manage')).map(([id]) => Number(id));
  const [people, setPeople] = useState<Person[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [catalog, setCatalog] = useState<Permission[]>([]);
  const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState('active');
  const [tab, setTab] = useState<'people' | 'roles'>('people');
  const [managingRolesFor, setManagingRolesFor] = useState<Person | null>(null);
  const [roleDialogError, setRoleDialogError] = useState('');
  const [editing, setEditing] = useState<Person | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState(''); const [email, setEmail] = useState(''); const [password, setPassword] = useState('');
  const [roleID, setRoleID] = useState(''); const [warehouseID, setWarehouseID] = useState('');
  const [roleName, setRoleName] = useState(''); const [roleScope, setRoleScope] = useState('warehouse'); const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]); const [editingRole, setEditingRole] = useState<Role | null>(null);
  const canSeeRoles = canManageRoles;
  async function load() {
    setLoading(true); setError('');
    try {
      const [personResult, roleResult, warehouseResult] = await Promise.all([
        request<{ people: Person[] }>(`/users?status=${status}&q=${encodeURIComponent(query)}`),
        request<{ roles: Role[] }>('/roles'),
        request<{ warehouses: Warehouse[] }>('/warehouses'),
      ]);
      setPeople(personResult.people ?? []); setRoles(roleResult.roles ?? []); setWarehouses(warehouseResult.warehouses ?? []);
      if (canManageRoles) { const permissionResult = await request<{ permissions: Permission[] }>('/permissions'); setCatalog(permissionResult.permissions ?? []); }
    } catch (e) { setError((e as ApiError).message); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); }, [status, query]);
  const allowedRoles = roles.filter((role) => role.slug !== 'super_admin' && (canManageAdmins || role.scope === 'warehouse') && (canManageCompany || role.slug === 'worker'));
  const availableWarehouses = warehouses.filter((item) => canManageCompany || managedWarehouseIds.includes(item.id));
  function createPerson() { setEditing(null); setName(''); setEmail(''); setPassword(''); setRoleID(''); setWarehouseID(''); setShowForm(true); setError(''); }
  function editPerson(person: Person) { setEditing(person); setName(person.name); setEmail(person.email); setPassword(''); setShowForm(true); setError(''); }
  async function savePerson(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError('');
    try {
      if (editing) await request(`/users/${editing.id}`, { method: 'PATCH', body: JSON.stringify({ name, email }) });
      else {
        const created = await request<Person>('/users', { method: 'POST', body: JSON.stringify({ name, email, password }) });
        setShowForm(false);
        await load();
        setManagingRolesFor(created);
        return;
      }
      setShowForm(false); await load();
    } catch (e) { setError((e as ApiError).message); }
  }
  async function deactivate(person: Person) {
    try { if (person.is_active) await request(`/users/${person.id}`, { method: 'DELETE' }); else await request(`/users/${person.id}`, { method: 'PATCH', body: JSON.stringify({ is_active: true }) }); await load(); }
    catch (e) { setError((e as ApiError).message); }
  }
  async function addAssignment(person: Person) {
    if (!roleID) return;
    const role = roles.find((r) => r.id === Number(roleID)); if (!role) return;
    try { await request(`/users/${person.id}/role-assignments`, { method: 'POST', body: JSON.stringify({ role_id: role.id, warehouse_id: role.scope === 'warehouse' ? Number(warehouseID) : null }) }); setRoleID(''); setWarehouseID(''); setRoleDialogError(''); await load(); }
    catch (e) { setRoleDialogError((e as ApiError).message); }
  }
  async function removeAssignment(person: Person, assignment: Assignment) {
    try { await request(`/users/${person.id}/role-assignments/${assignment.id}`, { method: 'DELETE' }); setRoleDialogError(''); await load(); } catch (e) { setRoleDialogError((e as ApiError).message); }
  }
  async function saveRole(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError('');
    try {
      if (editingRole) await request(`/roles/${editingRole.id}`, { method: 'PATCH', body: JSON.stringify({ name: roleName, permissions: selectedPermissions }) });
      else await request('/roles', { method: 'POST', body: JSON.stringify({ name: roleName, scope: roleScope, permissions: selectedPermissions }) });
      setEditingRole(null); setRoleName(''); setSelectedPermissions([]); await load();
    } catch (e) { setError((e as ApiError).message); }
  }
  async function removeRole(role: Role) { try { await request(`/roles/${role.id}`, { method: 'DELETE' }); await load(); } catch (e) { setError((e as ApiError).message); } }
  const roleModalPerson = managingRolesFor ? people.find((person) => person.id === managingRolesFor.id) ?? managingRolesFor : null;
  return <main className="workspace-shell"><header className="topbar"><div><span className="eyebrow">{user.company_slug}</span><strong>ProductionLineFlow</strong></div><button className="quiet-button" onClick={onBack}>← Overview</button></header><section className="workspace-content people-workspace"><div className="workspace-heading"><div><span className="panel-kicker">Company workspace</span><h1>People & roles</h1><p>Manage company access and warehouse memberships.</p></div></div>
    <div className="people-toolbar"><div className="people-tabs"><button className={tab === 'people' ? 'primary-button' : 'quiet-button'} onClick={() => setTab('people')}>People</button>{canSeeRoles && <button className={tab === 'roles' ? 'primary-button' : 'quiet-button'} onClick={() => setTab('roles')}>Roles</button>}</div>{tab === 'people' && <div className="people-filters"><input aria-label="Search people" placeholder="Search name or email" value={query} onChange={(e) => setQuery(e.target.value)} /><select value={status} onChange={(e) => setStatus(e.target.value)}><option value="active">Active</option><option value="inactive">Inactive</option><option value="all">All statuses</option></select>{(canManageCompany || managedWarehouseIds.length > 0) && <button className="primary-button" onClick={createPerson}>Add person</button>}</div>}</div>
    {error && <div className="form-error" role="alert">{error}</div>}
    {tab === 'people' && showForm && <form className="people-form" onSubmit={savePerson}><h2>{editing ? 'Edit person' : 'Add person'}</h2><label>Name<input required value={name} onChange={(e) => setName(e.target.value)} /></label><label>Company email<input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} /></label>{!editing && <label>Initial password<input type="password" minLength={10} required autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} /><small>Use at least 10 characters.</small></label>}<div className="people-actions"><button className="primary-button" type="submit">Save</button><button className="quiet-button" type="button" onClick={() => setShowForm(false)}>Cancel</button></div></form>}
    {loading ? <div className="empty-state">Loading people and roles…</div> : tab === 'people' ? people.length === 0 ? <div className="empty-state">No people match this filter.</div> : <div className="people-table-wrap"><table className="people-table"><thead><tr><th scope="col">Person</th><th scope="col">Current roles</th><th scope="col">Status</th><th scope="col">Actions</th></tr></thead><tbody>{people.map((person) => { const isManager = !canManageCompany; return <tr key={person.id}><td data-label="Person"><strong>{person.name}</strong><span className="people-email">{person.email}</span></td><td data-label="Current roles"><div className="assignment-list">{(person.assignments ?? []).length ? person.assignments.map((assignment) => <span className="assignment-chip" key={assignment.id}>{assignment.role_name}{assignment.warehouse_name ? ` · ${assignment.warehouse_name}` : ''}</span>) : <span className="muted-role">No roles assigned</span>}</div></td><td data-label="Status"><span className={person.is_active ? 'people-status active' : 'people-status'}>{person.is_active ? 'Active' : 'Inactive'}</span></td><td data-label="Actions"><div className="people-row-actions">{(allowedRoles.length > 0 || canManageAdmins) && person.is_active && <button className="quiet-button" onClick={() => { setManagingRolesFor(person); setRoleID(''); setWarehouseID(''); }}>Manage Role</button>}{canManageCompany && <PasswordReset person={person} />}{!isManager && <><button className="quiet-button" onClick={() => editPerson(person)}>Edit</button><button className={person.is_active ? 'quiet-button' : 'primary-button'} onClick={() => void deactivate(person)}>{person.is_active ? 'Deactivate' : 'Reactivate'}</button>{canManageAdmins && person.is_active && !(person.assignments ?? []).some((a) => a.role_slug === 'super_admin') && <button className="quiet-button" onClick={async () => { if (confirm(`Transfer Super Admin to ${person.name}?`)) { try { await request(`/users/${person.id}/transfer-super-admin`, { method: 'POST' }); await load(); } catch (e) { setError((e as ApiError).message); } } }}>Transfer Super Admin</button>}</>}</div></td></tr>; })}</tbody></table></div> : <><form className="people-form" onSubmit={saveRole}><h2>{editingRole ? 'Edit custom role' : 'Create custom role'}</h2><label>Role name<input required value={roleName} onChange={(e) => setRoleName(e.target.value)} /></label>{!editingRole && <label>Scope<select value={roleScope} onChange={(e) => setRoleScope(e.target.value)}><option value="warehouse">Warehouse</option><option value="company">Company</option></select></label>}<fieldset><legend>Permissions</legend><div className="permission-options">{catalog.map((permission) => <label key={permission.key}><input type="checkbox" checked={selectedPermissions.includes(permission.key)} onChange={(e) => setSelectedPermissions(e.target.checked ? [...selectedPermissions, permission.key] : selectedPermissions.filter((p) => p !== permission.key))} />{permission.description || permission.key}</label>)}</div></fieldset><div className="people-actions"><button className="primary-button" disabled={selectedPermissions.length === 0}>Save role</button>{editingRole && <button type="button" className="quiet-button" onClick={() => { setEditingRole(null); setRoleName(''); setSelectedPermissions([]); }}>Cancel</button>}</div></form><div className="people-list">{roles.map((role) => <article className="people-card" key={role.id}><div className="people-person"><h2>{role.name}</h2><span>{role.scope} · {role.is_system ? 'System role' : 'Custom role'}</span></div><p>{role.permissions.join(', ')}</p>{!role.is_system && <div className="people-actions"><button className="quiet-button" onClick={() => { setEditingRole(role); setRoleName(role.name); setSelectedPermissions(role.permissions); }}>Edit</button><button className="quiet-button" onClick={() => void removeRole(role)}>Delete</button></div>}</article>)}</div></>}
    {roleModalPerson && <div className="modal-backdrop" onMouseDown={(event) => { if (event.currentTarget === event.target) setManagingRolesFor(null); }}><section className="people-dialog" role="dialog" aria-modal="true" aria-labelledby="manage-role-title"><div className="dialog-heading"><div><span className="panel-kicker">Person access</span><h2 id="manage-role-title">Manage roles</h2><p>{roleModalPerson.name} · {roleModalPerson.email}</p></div><button className="dialog-close" aria-label="Close manage roles" onClick={() => setManagingRolesFor(null)}>×</button></div><div className="dialog-section"><h3>Current roles</h3>{(roleModalPerson.assignments ?? []).length === 0 ? <p className="muted-role">No roles assigned yet.</p> : <ul className="dialog-role-list">{roleModalPerson.assignments.map((assignment) => <li key={assignment.id}><span><strong>{assignment.role_name}</strong>{assignment.warehouse_name && <small>{assignment.warehouse_name}</small>}</span>{assignment.role_slug !== 'super_admin' && (canManageAdmins || (!canManageCompany && assignment.role_slug === 'worker' && managedWarehouseIds.includes(assignment.warehouse_id ?? -1))) && <button className="quiet-button" onClick={() => void removeAssignment(roleModalPerson, assignment)}>Remove</button>}</li>)}</ul>}</div>{allowedRoles.length > 0 && roleModalPerson.is_active && <div className="dialog-section"><h3>Assign a role</h3><div className="role-assignment-form"><label>Role<select value={roleID} onChange={(event) => { setRoleID(event.target.value); setWarehouseID(''); }}><option value="">Choose a role</option>{allowedRoles.map((role) => <option key={role.id} value={role.id}>{role.name} · {role.scope}</option>)}</select></label>{roleID && roles.find((role) => role.id === Number(roleID))?.scope === 'warehouse' && <label>Warehouse<select value={warehouseID} onChange={(event) => setWarehouseID(event.target.value)}><option value="">Choose a warehouse</option>{availableWarehouses.map((warehouse) => <option key={warehouse.id} value={warehouse.id}>{warehouse.name}</option>)}</select></label>}<button className="primary-button" disabled={!roleID || (roles.find((role) => role.id === Number(roleID))?.scope === 'warehouse' && !warehouseID)} onClick={() => void addAssignment(roleModalPerson)}>Assign role</button></div>{roleDialogError && <div className="form-error" role="alert">{roleDialogError}</div>}</div>}<div className="dialog-footer"><button className="quiet-button" onClick={() => setManagingRolesFor(null)}>Done</button></div></section></div>}
  </section></main>;
}

function PasswordReset({ person }: { person: Person }) {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  async function resetPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage('');
    setError('');
    if (password !== confirmation) {
      setError('The passwords do not match.');
      return;
    }
    setSaving(true);
    try {
      await request(`/users/${person.id}/password`, { method: 'PATCH', body: JSON.stringify({ password }) });
      setPassword('');
      setConfirmation('');
      setMessage('Password reset successfully.');
    } catch (resetError) {
      setError((resetError as ApiError).message);
    } finally {
      setSaving(false);
    }
  }

  return <><button className="quiet-button" onClick={() => { setError(''); setMessage(''); setOpen(true); }}>Manage Password</button>{open && <div className="modal-backdrop" onMouseDown={(event) => { if (event.currentTarget === event.target && !saving) setOpen(false); }}><section className="people-dialog" role="dialog" aria-modal="true" aria-labelledby={`password-reset-title-${person.id}`}><div className="dialog-heading"><div><span className="panel-kicker">Account security</span><h2 id={`password-reset-title-${person.id}`}>Manage password</h2><p>Set a new password for {person.name}.</p></div><button className="dialog-close" aria-label="Close manage password" disabled={saving} onClick={() => setOpen(false)}>×</button></div><form className="password-reset-form" onSubmit={resetPassword}><label>New Password<input type="password" autoComplete="new-password" minLength={10} required value={password} onChange={(event) => setPassword(event.target.value)} /></label><label>Confirm Password<input type="password" autoComplete="new-password" minLength={10} required value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label><small>Use at least 10 characters.</small>{error && <div className="form-error" role="alert">{error}</div>}{message && <div className="success-message" role="status">{message}</div>}<div className="dialog-footer"><button className="quiet-button" type="button" disabled={saving} onClick={() => setOpen(false)}>Cancel</button><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Resetting…' : 'Reset Password'}</button></div></form></section></div>}</>;
}
