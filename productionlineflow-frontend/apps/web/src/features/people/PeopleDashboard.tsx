import { useEffect, useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import type { ApiError, Person, Permission, Permissions, Role, User, Warehouse } from '../../types';
import PersonManageDialog from './PersonManageDialog';

export default function PeopleDashboard({ user, permissions, onTransferred }: { user: User; permissions: Permissions; onTransferred: () => Promise<void> }) {
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
  const [status, setStatus] = useState('all');
  const [tab, setTab] = useState<'people' | 'roles'>('people');
  const [managing, setManaging] = useState<Person | null>(null);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [roleName, setRoleName] = useState('');
  const [roleScope, setRoleScope] = useState('warehouse');
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const [editingRole, setEditingRole] = useState<Role | null>(null);

  async function load() {
    setLoading(true); setError('');
    try {
      const [personResult, roleResult, warehouseResult] = await Promise.all([
        request<{ people: Person[] }>(`/users?status=${status}&q=${encodeURIComponent(query)}`),
        request<{ roles: Role[] }>('/roles'),
        request<{ warehouses: Warehouse[] }>('/warehouses'),
      ]);
      setPeople(personResult.people ?? []); setRoles(roleResult.roles ?? []); setWarehouses(warehouseResult.warehouses ?? []);
      if (canManageRoles) { const result = await request<{ permissions: Permission[] }>('/permissions'); setCatalog(result.permissions ?? []); }
    } catch (e) { setError((e as ApiError).message); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); }, [status, query]);

  async function createPerson(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError('');
    try {
      const created = await request<Person>('/users', { method: 'POST', body: JSON.stringify({ name, email, password }) });
      setAdding(false); setName(''); setEmail(''); setPassword(''); await load(); setManaging(created);
    } catch (e) { setError((e as ApiError).message); }
  }
  async function toggleActive(person: Person) {
    setError('');
    try {
      if (person.is_active) await request(`/users/${person.id}`, { method: 'DELETE' });
      else await request(`/users/${person.id}`, { method: 'PATCH', body: JSON.stringify({ is_active: true }) });
      await load();
    } catch (e) { setError((e as ApiError).message); }
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
  const manageablePeople = people;

  return <main className="workspace-shell">
    <section className="workspace-content people-workspace"><div className="workspace-heading"><div><span className="panel-kicker">Company workspace</span><h1>People &amp; roles</h1><p>Manage company access and warehouse memberships.</p></div></div>
      <div className="people-toolbar"><div className="people-tabs"><button className={tab === 'people' ? 'primary-button' : 'quiet-button'} onClick={() => setTab('people')}>People</button>{canManageRoles && <button className={tab === 'roles' ? 'primary-button' : 'quiet-button'} onClick={() => setTab('roles')}>Roles</button>}</div>
        {tab === 'people' && <div className="people-filters"><input aria-label="Search people" placeholder="Search name or email" value={query} onChange={(event) => setQuery(event.target.value)} /><select aria-label="Filter by status" value={status} onChange={(event) => setStatus(event.target.value)}><option value="all">All statuses</option><option value="active">Active</option><option value="inactive">Inactive</option></select>{canManageCompany && <button className="primary-button" onClick={() => { setError(''); setAdding(true); }}>Add person</button>}</div>}
      </div>
      {error && <div className="form-error" role="alert">{error}</div>}
      {loading ? <div className="empty-state">Loading people and roles…</div> : tab === 'people' ? manageablePeople.length === 0 ? <div className="empty-state">No people match this filter.</div> : <div className="people-table-wrap"><table className="people-table"><thead><tr><th scope="col">Person</th><th scope="col">Current roles</th><th scope="col">Status</th><th scope="col">Actions</th></tr></thead><tbody>{manageablePeople.map((person) => {
        const privileged = (person.assignments ?? []).some((assignment) => ['admin', 'super_admin'].includes(assignment.role_slug));
        const canToggle = canManageCompany && (!privileged || canManageAdmins);
        const canOpenManage = canManageCompany || canManageAdmins || managedWarehouseIds.length > 0;
        return <tr key={person.id}><td data-label="Person"><strong>{person.name}</strong><span className="people-email">{person.email}</span></td><td data-label="Current roles"><div className="assignment-list">{(person.assignments ?? []).length ? person.assignments.map((assignment) => <span className="assignment-chip" key={assignment.id}>{assignment.role_name}{assignment.warehouse_name ? ` · ${assignment.warehouse_name}` : ''}</span>) : <span className="muted-role">No roles assigned</span>}</div></td><td data-label="Status"><span className={person.is_active ? 'people-status active' : 'people-status'}>{person.is_active ? 'Active' : 'Inactive'}</span></td><td data-label="Actions"><div className="people-row-actions">{canOpenManage && <button className="quiet-button" onClick={() => setManaging(person)}>Manage</button>}{canToggle && <button className={person.is_active ? 'quiet-button' : 'primary-button'} onClick={() => void toggleActive(person)}>{person.is_active ? 'Deactivate' : 'Reactivate'}</button>}</div></td></tr>;
      })}</tbody></table></div> : <>
        <form className="people-form" onSubmit={saveRole}><h2>{editingRole ? 'Edit custom role' : 'Create custom role'}</h2><label>Role name<input required value={roleName} onChange={(event) => setRoleName(event.target.value)} /></label>{!editingRole && <label>Scope<select value={roleScope} onChange={(event) => setRoleScope(event.target.value)}><option value="warehouse">Warehouse</option><option value="company">Company</option></select></label>}<fieldset><legend>Permissions</legend><div className="permission-options">{catalog.map((permission) => <label key={permission.key}><input type="checkbox" checked={selectedPermissions.includes(permission.key)} onChange={(event) => setSelectedPermissions(event.target.checked ? [...selectedPermissions, permission.key] : selectedPermissions.filter((item) => item !== permission.key))} />{permission.description || permission.key}</label>)}</div></fieldset><div className="people-actions"><button className="primary-button" disabled={selectedPermissions.length === 0}>Save role</button>{editingRole && <button type="button" className="quiet-button" onClick={() => { setEditingRole(null); setRoleName(''); setSelectedPermissions([]); }}>Cancel</button>}</div></form>
        <div className="people-list">{roles.map((role) => <article className="people-card" key={role.id}><div className="people-person"><h2>{role.name}</h2><span>{role.scope} · {role.is_system ? 'System role' : 'Custom role'}</span></div><p>{role.permissions.join(', ')}</p>{!role.is_system && <div className="people-actions"><button className="quiet-button" onClick={() => { setEditingRole(role); setRoleName(role.name); setSelectedPermissions(role.permissions); }}>Edit</button><button className="quiet-button" onClick={() => void removeRole(role)}>Delete</button></div>}</article>)}</div>
      </>}
    </section>
    {adding && <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) setAdding(false); }}><section className="people-dialog" role="dialog" aria-modal="true" aria-labelledby="add-person-title"><div className="dialog-heading"><div><span className="panel-kicker">Company access</span><h2 id="add-person-title">Add person</h2><p>Create an account, then use Manage to assign roles.</p></div><button className="dialog-close" aria-label="Close add person" onClick={() => setAdding(false)}>×</button></div><form className="person-edit-form" onSubmit={createPerson}><label>Name<input required value={name} onChange={(event) => setName(event.target.value)} /></label><label>Company email<input type="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label><label>Initial password<input type="password" minLength={10} required autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} /><small>Use at least 10 characters.</small></label><div className="dialog-footer"><button type="button" className="quiet-button" onClick={() => setAdding(false)}>Cancel</button><button className="primary-button" type="submit">Create person</button></div></form></section></div>}
    {managing && <PersonManageDialog person={managing} roles={roles} warehouses={warehouses} canManageCompany={canManageCompany} canManageAdmins={canManageAdmins} managedWarehouseIds={managedWarehouseIds} onClose={() => setManaging(null)} onTransferred={onTransferred} onUpdated={async () => { await load(); }} />}
  </main>;
}
