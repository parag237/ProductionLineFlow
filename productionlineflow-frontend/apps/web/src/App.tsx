import { FormEvent, useEffect, useState } from 'react';

type User = {
  id: number;
  name: string;
  email: string;
  company_id: number;
  company_slug: string;
};

type Permissions = {
  company: string[];
  warehouses: Record<string, string[]>;
};

type SessionResponse = {
  access_token: string;
  expires_in: number;
  user: User;
  permissions: Permissions;
};

type MeResponse = {
  user: User;
  permissions: Permissions;
};
type Warehouse = { id: number; name: string; type_name: string; address?: string; state: string; created_at: string };
type Person = { id: number; name: string; email: string; is_active: boolean; assignments: Assignment[] };
type Assignment = { id: number; role_id: number; role_slug: string; role_name: string; role_scope: string; warehouse_id: number | null; warehouse_name?: string };
type Role = { id: number; slug: string; name: string; scope: string; is_system: boolean; permissions: string[] };
type Permission = { key: string; description: string };

type ApiError = Error & { code?: string; status?: number };

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';

let accessToken = '';
let platformAccessToken = '';

function markFormSubmitted(event: FormEvent<HTMLFormElement>) {
  event.currentTarget.classList.add('form-submitted');
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  });

  if (response.status === 401 && retry && path !== '/auth/refresh' && path !== '/auth/login') {
    try {
      const refreshed = await request<SessionResponse>('/auth/refresh', { method: 'POST' }, false);
      accessToken = refreshed.access_token;
      return request<T>(path, init, false);
    } catch {
      accessToken = '';
    }
  }

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: { code?: string; message?: string } } | null;
    const error = new Error(body?.error?.message ?? 'Request failed') as ApiError;
    error.code = body?.error?.code;
    error.status = response.status;
    throw error;
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

type PlatformUser = { id: number; name: string; email: string };
type PlatformSession = { access_token: string; user: PlatformUser; permissions: string[] };
type PlatformCompany = { id: number; slug: string; name: string; status: 'active' | 'suspended'; suspended_at?: string; activated_at?: string };

async function platformRequest<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  if (platformAccessToken) headers.set('Authorization', `Bearer ${platformAccessToken}`);
  const response = await fetch(`${API_BASE}/platform${path}`, { ...init, headers, credentials: 'include' });
  if (response.status === 401 && retry && path !== '/auth/refresh' && path !== '/auth/login') {
    try {
      const refreshed = await platformRequest<PlatformSession>('/auth/refresh', { method: 'POST' }, false);
      platformAccessToken = refreshed.access_token;
      return platformRequest<T>(path, init, false);
    } catch {
      platformAccessToken = '';
    }
  }
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: { code?: string; message?: string } } | null;
    const error = new Error(body?.error?.message ?? 'Request failed') as ApiError;
    error.code = body?.error?.code;
    error.status = response.status;
    throw error;
  }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>;
}

function LoginView({ onLogin }: { onLogin: (session: SessionResponse) => void }) {
  const [companySlug, setCompanySlug] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      const session = await request<SessionResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ company_slug: companySlug, email, password }),
      });
      accessToken = session.access_token;
      onLogin(session);
      setPassword('');
    } catch (loginError) {
      const typedError = loginError as ApiError;
      setError(typedError.code === 'invalid_credentials' ? 'The company, email, or password is not valid.' : typedError.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-layout">
      <section className="auth-intro">
        <span className="eyebrow">ProductionLineFlow</span>
        <h1>Keep every warehouse moving.</h1>
        <p>Sign in to coordinate people, places, and production work from one operational view.</p>
      </section>
      <section className="login-panel" aria-labelledby="login-title">
        <div className="panel-heading">
          <span className="panel-kicker">Workspace access</span>
          <h2 id="login-title">Sign in</h2>
          <p>Use your company workspace credentials.</p>
        </div>
        <form onSubmit={submit} onInvalid={markFormSubmitted}>
          <label>
            Company workspace
            <input value={companySlug} onChange={(event) => setCompanySlug(event.target.value)} autoComplete="organization" required placeholder="acme" />
          </label>
          <label>
            Email
            <input type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username" required placeholder="you@company.com" />
          </label>
          <label>
            Password
            <input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required />
          </label>
          {error && <div className="form-error" role="alert">{error}</div>}
          <button className="primary-button" type="submit" disabled={submitting}>
            {submitting ? 'Signing in...' : 'Sign in'}
          </button>
        </form>
      </section>
    </main>
  );
}

function Workspace({ user, permissions, onLogout, onOpenWarehouses, onOpenPeople }: { user: User; permissions: Permissions; onLogout: () => void; onOpenWarehouses: () => void; onOpenPeople: () => void }) {
  const navigation = [
    { label: 'Warehouses', permission: 'warehouse.view' },
    { label: 'People & roles', permission: 'users.create' },
    { label: 'Operations', permission: 'workers.tasks.execute' },
  ].filter((item) => item.label === 'People & roles' ? permissions.company.includes('users.create') || Object.values(permissions.warehouses).some((list) => list.includes('warehouse.members.manage')) : permissions.company.includes(item.permission));

  return (
    <main className="workspace-shell">
      <header className="topbar">
        <div>
          <span className="eyebrow">{user.company_slug}</span>
          <strong>ProductionLineFlow</strong>
        </div>
        <div className="user-menu">
          <span>{user.name}</span>
          <button className="quiet-button" onClick={onLogout}>Sign out</button>
        </div>
      </header>
      <section className="workspace-content">
        <div className="workspace-heading">
          <div>
            <span className="panel-kicker">Operations overview</span>
            <h1>Good to see you, {user.name.split(' ')[0]}.</h1>
            <p>Your access is ready. Choose a workspace area to continue.</p>
          </div>
          <div className="status-mark"><span /> Session active</div>
        </div>
        <div className="workspace-grid">
          {navigation.length > 0 ? navigation.map((item) => (
            <button className="workspace-card" key={item.label} onClick={item.label === 'Warehouses' ? onOpenWarehouses : item.label === 'People & roles' ? onOpenPeople : undefined}>
              <span className="card-arrow">↗</span>
              <span className="card-label">{item.label}</span>
              <span className="card-meta">Available to your role</span>
            </button>
          )) : <div className="empty-state">Your account is authenticated, but no workspace permissions are assigned yet.</div>}
        </div>
      </section>
    </main>
  );
}

function PeopleDashboard({ user, permissions, onBack }: { user: User; permissions: Permissions; onBack: () => void }) {
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
        const assignments = roleID ? [{ role_id: Number(roleID), warehouse_id: roles.find((r) => r.id === Number(roleID))?.scope === 'warehouse' ? Number(warehouseID) : null }] : [];
        await request('/users', { method: 'POST', body: JSON.stringify({ name, email, password, assignments }) });
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
    try { await request(`/users/${person.id}/role-assignments`, { method: 'POST', body: JSON.stringify({ role_id: role.id, warehouse_id: role.scope === 'warehouse' ? Number(warehouseID) : null }) }); setRoleID(''); setWarehouseID(''); await load(); }
    catch (e) { setError((e as ApiError).message); }
  }
  async function removeAssignment(person: Person, assignment: Assignment) {
    try { await request(`/users/${person.id}/role-assignments/${assignment.id}`, { method: 'DELETE' }); await load(); } catch (e) { setError((e as ApiError).message); }
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
  return <main className="workspace-shell"><header className="topbar"><div><span className="eyebrow">{user.company_slug}</span><strong>ProductionLineFlow</strong></div><button className="quiet-button" onClick={onBack}>← Overview</button></header><section className="workspace-content people-workspace"><div className="workspace-heading"><div><span className="panel-kicker">Company workspace</span><h1>People & roles</h1><p>Manage company access and warehouse memberships.</p></div></div>
    <div className="people-toolbar"><div className="people-tabs"><button className={tab === 'people' ? 'primary-button' : 'quiet-button'} onClick={() => setTab('people')}>People</button>{canSeeRoles && <button className={tab === 'roles' ? 'primary-button' : 'quiet-button'} onClick={() => setTab('roles')}>Roles</button>}</div>{tab === 'people' && <div className="people-filters"><input aria-label="Search people" placeholder="Search name or email" value={query} onChange={(e) => setQuery(e.target.value)} /><select value={status} onChange={(e) => setStatus(e.target.value)}><option value="active">Active</option><option value="inactive">Inactive</option><option value="all">All statuses</option></select>{(canManageCompany || managedWarehouseIds.length > 0) && <button className="primary-button" onClick={createPerson}>Add person</button>}</div>}</div>
    {error && <div className="form-error" role="alert">{error}</div>}
    {tab === 'people' && showForm && <form className="people-form" onSubmit={savePerson}><h2>{editing ? 'Edit person' : 'Add person'}</h2><label>Name<input required value={name} onChange={(e) => setName(e.target.value)} /></label><label>Company email<input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} /></label>{!editing && <><label>Initial password<input type="password" minLength={8} required value={password} onChange={(e) => setPassword(e.target.value)} /></label><label>Role<select value={roleID} onChange={(e) => setRoleID(e.target.value)} required><option value="">Choose a role</option>{allowedRoles.map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}</select></label>{roleID && roles.find((r) => r.id === Number(roleID))?.scope === 'warehouse' && <label>Warehouse<select required value={warehouseID} onChange={(e) => setWarehouseID(e.target.value)}><option value="">Choose a warehouse</option>{availableWarehouses.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}</select></label>}</>}<div className="people-actions"><button className="primary-button" type="submit">Save</button><button className="quiet-button" type="button" onClick={() => setShowForm(false)}>Cancel</button></div></form>}
    {loading ? <div className="empty-state">Loading people and roles…</div> : tab === 'people' ? people.length === 0 ? <div className="empty-state">No people match this filter.</div> : <div className="people-list">{people.map((person) => { const isManager = !canManageCompany; return <article className="people-card" key={person.id}><div className="people-person"><div><h2>{person.name}</h2><p>{person.email}</p></div><span className={person.is_active ? 'people-status active' : 'people-status'}>{person.is_active ? 'Active' : 'Inactive'}</span></div><div className="assignment-list">{person.assignments.map((assignment) => <span className="assignment-chip" key={assignment.id}>{assignment.role_name}{assignment.warehouse_name ? ` · ${assignment.warehouse_name}` : ''}{(canManageAdmins || (isManager && assignment.role_slug === 'worker' && managedWarehouseIds.includes(assignment.warehouse_id ?? -1))) && assignment.role_slug !== 'super_admin' && <button aria-label={`Remove ${assignment.role_name} assignment`} onClick={() => void removeAssignment(person, assignment)}>×</button>}</span>)}</div>{allowedRoles.length > 0 && person.is_active && <div className="people-actions"><select aria-label="Role to assign" value={roleID} onChange={(e) => setRoleID(e.target.value)}><option value="">Add assignment…</option>{allowedRoles.map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}</select>{roleID && roles.find((r) => r.id === Number(roleID))?.scope === 'warehouse' && <select aria-label="Assignment warehouse" value={warehouseID} onChange={(e) => setWarehouseID(e.target.value)}><option value="">Choose warehouse</option>{availableWarehouses.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}</select>}<button className="quiet-button" disabled={!roleID || (roles.find((r) => r.id === Number(roleID))?.scope === 'warehouse' && !warehouseID)} onClick={() => void addAssignment(person)}>Assign role</button></div>}{!isManager && <div className="people-actions"><button className="quiet-button" onClick={() => editPerson(person)}>Edit</button><button className="quiet-button" onClick={() => void deactivate(person)}>{person.is_active ? 'Deactivate' : 'Reactivate'}</button>{canManageCompany && <PasswordReset person={person} />}{canManageAdmins && person.is_active && !person.assignments.some((a) => a.role_slug === 'super_admin') && <button className="quiet-button" onClick={async () => { if (confirm(`Transfer Super Admin to ${person.name}?`)) { try { await request(`/users/${person.id}/transfer-super-admin`, { method: 'POST' }); await load(); } catch (e) { setError((e as ApiError).message); } } }}>Transfer Super Admin</button>}</div>}</article>; })}</div> : <><form className="people-form" onSubmit={saveRole}><h2>{editingRole ? 'Edit custom role' : 'Create custom role'}</h2><label>Role name<input required value={roleName} onChange={(e) => setRoleName(e.target.value)} /></label>{!editingRole && <label>Scope<select value={roleScope} onChange={(e) => setRoleScope(e.target.value)}><option value="warehouse">Warehouse</option><option value="company">Company</option></select></label>}<fieldset><legend>Permissions</legend><div className="permission-options">{catalog.map((permission) => <label key={permission.key}><input type="checkbox" checked={selectedPermissions.includes(permission.key)} onChange={(e) => setSelectedPermissions(e.target.checked ? [...selectedPermissions, permission.key] : selectedPermissions.filter((p) => p !== permission.key))} />{permission.description || permission.key}</label>)}</div></fieldset><div className="people-actions"><button className="primary-button" disabled={selectedPermissions.length === 0}>Save role</button>{editingRole && <button type="button" className="quiet-button" onClick={() => { setEditingRole(null); setRoleName(''); setSelectedPermissions([]); }}>Cancel</button>}</div></form><div className="people-list">{roles.map((role) => <article className="people-card" key={role.id}><div className="people-person"><h2>{role.name}</h2><span>{role.scope} · {role.is_system ? 'System role' : 'Custom role'}</span></div><p>{role.permissions.join(', ')}</p>{!role.is_system && <div className="people-actions"><button className="quiet-button" onClick={() => { setEditingRole(role); setRoleName(role.name); setSelectedPermissions(role.permissions); }}>Edit</button><button className="quiet-button" onClick={() => void removeRole(role)}>Delete</button></div>}</article>)}</div></>}
  </section></main>;
}

function PasswordReset({ person }: { person: Person }) {
  const [password, setPassword] = useState(''); const [message, setMessage] = useState('');
  return <form className="inline-password" onSubmit={async (event) => { event.preventDefault(); try { await request(`/users/${person.id}/password`, { method: 'PATCH', body: JSON.stringify({ password }) }); setPassword(''); setMessage('Password updated'); } catch (e) { setMessage((e as ApiError).message); } }}><input type="password" minLength={8} required placeholder="New password" value={password} onChange={(e) => setPassword(e.target.value)} /><button className="quiet-button">Reset password</button>{message && <small>{message}</small>}</form>;
}

function WarehouseDashboard({ canManage, onBack }: { canManage: boolean; onBack: () => void }) {
  const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<Warehouse | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [typeName, setTypeName] = useState('');
  const [address, setAddress] = useState('');

  async function loadWarehouses() {
    setLoading(true);
    try {
      const result = await request<{ warehouses: Warehouse[] | null }>('/warehouses');
      setWarehouses(result.warehouses ?? []);
    } catch (loadError) { setError((loadError as ApiError).message); }
    finally { setLoading(false); }
  }

  useEffect(() => { loadWarehouses(); }, []);

  function startCreate() { setEditing(null); setName(''); setTypeName(''); setAddress(''); setError(''); setShowForm(true); }
  function startEdit(item: Warehouse) { setEditing(item); setName(item.name); setTypeName(item.type_name); setAddress(item.address ?? ''); setError(''); setShowForm(true); }

  async function saveWarehouse(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      const payload = JSON.stringify({ name, type_name: typeName, address });
      if (editing) await request(`/warehouses/${editing.id}`, { method: 'PATCH', body: payload });
      else await request('/warehouses', { method: 'POST', body: payload });
      setShowForm(false);
      await loadWarehouses();
    } catch (saveError) { setError((saveError as ApiError).message); }
  }

  async function deleteWarehouse(id: number) {
    if (!window.confirm('Delete this warehouse?')) return;
    try { await request(`/warehouses/${id}`, { method: 'DELETE' }); await loadWarehouses(); }
    catch (deleteError) { setError((deleteError as ApiError).message); }
  }

  return <main className="workspace-shell"><header className="topbar"><div><span className="eyebrow">Warehouse operations</span><strong>ProductionLineFlow</strong></div><button className="quiet-button" onClick={onBack}>Back to overview</button></header><section className="workspace-content"><div className="workspace-heading"><div><span className="panel-kicker">Company warehouses</span><h1>Warehouse dashboard.</h1><p>Manage every operating location in this company.</p></div>{canManage && <button className="primary-button compact-button" onClick={startCreate}>Add warehouse</button>}</div>{error && <div className="form-error" role="alert">{error}</div>}{showForm && <section className="login-panel onboarding-panel"><div className="panel-heading"><span className="panel-kicker">{editing ? 'Edit location' : 'New location'}</span><h2>{editing ? 'Update warehouse' : 'Add warehouse'}</h2></div><form onSubmit={saveWarehouse} onInvalid={markFormSubmitted}><label>Warehouse name<input value={name} onChange={(event) => setName(event.target.value)} required /></label><label>Warehouse type<input value={typeName} onChange={(event) => setTypeName(event.target.value)} required placeholder="Factory" /></label><label>Address<input value={address} onChange={(event) => setAddress(event.target.value)} /></label><div className="company-actions"><button className="primary-button compact-button" type="submit">{editing ? 'Save changes' : 'Create warehouse'}</button><button className="quiet-button" type="button" onClick={() => setShowForm(false)}>Cancel</button></div></form></section>}<div className="warehouse-list">{loading ? <div className="directory-message">Loading warehouses...</div> : warehouses.length === 0 ? <div className="empty-state">No warehouses yet. Add the first operating location.</div> : warehouses.map((item) => <article className="warehouse-row" key={item.id}><div><strong>{item.name}</strong><span>{item.type_name} · {item.address || 'No address'}</span></div><span className="status-badge active">{item.state}</span>{canManage && <div className="company-actions"><button className="quiet-button" onClick={() => startEdit(item)}>Edit</button><button className="danger-button" onClick={() => deleteWarehouse(item.id)}>Delete</button></div>}</article>)}</div></section></main>;
}

function PlatformLogin({ onLogin }: { onLogin: (session: PlatformSession) => void }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      const session = await platformRequest<PlatformSession>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      });
      platformAccessToken = session.access_token;
      setPassword('');
      onLogin(session);
    } catch (loginError) {
      const typedError = loginError as ApiError;
      setError(typedError.code === 'invalid_platform_credentials' ? 'The platform credentials are not valid.' : typedError.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-layout">
      <section className="auth-intro">
        <span className="eyebrow">Platform operations</span>
        <h1>Build the next workspace.</h1>
        <p>Product Owner access for creating and provisioning ProductionLineFlow companies.</p>
      </section>
      <section className="login-panel" aria-labelledby="platform-login-title">
        <div className="panel-heading">
          <span className="panel-kicker">Operator access</span>
          <h2 id="platform-login-title">Platform sign in</h2>
          <p>This area is separate from tenant warehouse workspaces.</p>
        </div>
        <form onSubmit={submit} onInvalid={markFormSubmitted}>
          <label>Email<input type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username" required /></label>
          <label>Password<input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required /></label>
          {error && <div className="form-error" role="alert">{error}</div>}
          <button className="primary-button" type="submit" disabled={submitting}>{submitting ? 'Signing in...' : 'Enter platform'}</button>
        </form>
      </section>
    </main>
  );
}

function PlatformWorkspace({ user, permissions, onLogout }: { user: PlatformUser; permissions: string[]; onLogout: () => void }) {
  const canCreateCompany = permissions.includes('companies.create');
  const [showCreate, setShowCreate] = useState(false);
  const [companyName, setCompanyName] = useState('');
  const [companySlug, setCompanySlug] = useState('');
  const [adminName, setAdminName] = useState('');
  const [adminEmail, setAdminEmail] = useState('');
  const [adminPassword, setAdminPassword] = useState('');
  const [adminPasswordConfirmation, setAdminPasswordConfirmation] = useState('');
  const [createError, setCreateError] = useState('');
  const [createdCompany, setCreatedCompany] = useState<{ Name: string; Slug: string; SuperAdminEmail: string } | null>(null);
  const [companies, setCompanies] = useState<PlatformCompany[]>([]);
  const [companiesLoading, setCompaniesLoading] = useState(false);
  const [companyFilter, setCompanyFilter] = useState<'all' | 'active' | 'suspended'>('all');
  const [suspendingCompanyId, setSuspendingCompanyId] = useState<number | null>(null);
  const [editingCompany, setEditingCompany] = useState<PlatformCompany | null>(null);
  const [editName, setEditName] = useState('');
  const [editSlug, setEditSlug] = useState('');
  const [managementError, setManagementError] = useState('');

  async function refreshCompanies() {
    if (!permissions.includes('companies.manage')) return;
    setCompaniesLoading(true);
    try {
      const result = await platformRequest<{ companies: PlatformCompany[] }>('/companies');
      setCompanies(result.companies);
    } finally {
      setCompaniesLoading(false);
    }
  }

  useEffect(() => {
    refreshCompanies().catch((error) => setManagementError((error as ApiError).message));
  }, [permissions]);

  async function createCompany(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCreateError('');
    if (adminPassword !== adminPasswordConfirmation) {
      setCreateError('The Super Admin passwords must match.');
      return;
    }
    try {
      const result = await platformRequest<{ Name: string; Slug: string; SuperAdminEmail: string }>('/companies', {
        method: 'POST',
        body: JSON.stringify({ name: companyName, slug: companySlug, super_admin: { name: adminName, email: adminEmail, password: adminPassword } }),
      });
      setCreatedCompany(result);
      setAdminPassword('');
      setAdminPasswordConfirmation('');
      await refreshCompanies();
    } catch (error) {
      setCreateError((error as ApiError).message);
    }
  }

  function beginEdit(item: PlatformCompany) {
    setEditingCompany(item);
    setEditName(item.name);
    setEditSlug(item.slug);
    setManagementError('');
  }

  async function updateCompany(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingCompany) return;
    try {
      await platformRequest(`/companies/${editingCompany.id}`, { method: 'PATCH', body: JSON.stringify({ name: editName, slug: editSlug }) });
      setEditingCompany(null);
      await refreshCompanies();
    } catch (error) { setManagementError((error as ApiError).message); }
  }

  async function suspendCompany(id: number) {
    setSuspendingCompanyId(id);
    try {
      await platformRequest(`/companies/${id}/suspend`, { method: 'POST' });
      await refreshCompanies();
    } catch (error) { setManagementError((error as ApiError).message); }
    finally { setSuspendingCompanyId(null); }
  }

  async function reactivateCompany(id: number) {
    setSuspendingCompanyId(id);
    try {
      await platformRequest(`/companies/${id}/reactivate`, { method: 'POST' });
      await refreshCompanies();
    } catch (error) { setManagementError((error as ApiError).message); }
    finally { setSuspendingCompanyId(null); }
  }

  function formatLifecycleTime(value?: string) {
    return value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : 'Not recorded';
  }

  const filteredCompanies = companies.filter((item) => companyFilter === 'all' || item.status === companyFilter);
  const activeCompanyCount = companies.filter((item) => item.status === 'active').length;
  const suspendedCompanyCount = companies.filter((item) => item.status === 'suspended').length;

  return (
    <main className="workspace-shell platform-shell">
      <header className="topbar"><div><span className="eyebrow">Platform operations</span><strong>ProductionLineFlow</strong></div><div className="user-menu"><span>{user.name}</span><button className="quiet-button" onClick={onLogout}>Sign out</button></div></header>
      <section className="workspace-content">
        <div className="workspace-heading"><div><span className="panel-kicker">Operator console</span><h1>Company control room.</h1><p>Provision and manage tenant workspaces from the platform boundary.</p></div><div className="status-mark"><span /> Platform session active</div></div>
        <div className="workspace-grid">
          {canCreateCompany && !showCreate && !createdCompany && <button className="workspace-card" onClick={() => setShowCreate(true)}><span className="card-arrow">＋</span><span className="card-label">Create company</span><span className="card-meta">Provision a company and its first Super Admin</span></button>}
          {!canCreateCompany && <div className="empty-state">This platform account has no company-management permissions.</div>}
        </div>
        {showCreate && !createdCompany && <section className="login-panel onboarding-panel"><div className="panel-heading"><span className="panel-kicker">New tenant workspace</span><h2>Create company</h2><p>Provision the company and its first Super Admin in one transaction.</p></div><form onSubmit={createCompany} onInvalid={markFormSubmitted}><label>Company name<input value={companyName} onChange={(event) => setCompanyName(event.target.value)} required /></label><label>Company slug<input value={companySlug} onChange={(event) => setCompanySlug(event.target.value)} pattern="[a-z0-9]+(?:-[a-z0-9]+)*" required /></label><label>Super Admin name<input value={adminName} onChange={(event) => setAdminName(event.target.value)} required /></label><label>Super Admin email<input type="email" value={adminEmail} onChange={(event) => setAdminEmail(event.target.value)} required /></label><label>Temporary password<input type="password" value={adminPassword} onChange={(event) => setAdminPassword(event.target.value)} minLength={10} required /></label><label>Confirm password<input type="password" value={adminPasswordConfirmation} onChange={(event) => setAdminPasswordConfirmation(event.target.value)} minLength={10} required /></label>{createError && <div className="form-error" role="alert">{createError}</div>}<button className="primary-button" type="submit">Provision company</button></form></section>}
        {createdCompany && <section className="login-panel onboarding-panel"><span className="panel-kicker">Provisioning complete</span><h2>{createdCompany.Name}</h2><p>Workspace <strong>{createdCompany.Slug}</strong> is ready. The initial Super Admin is {createdCompany.SuperAdminEmail}.</p><button className="quiet-button" onClick={() => { setCreatedCompany(null); setShowCreate(false); }}>Create another company</button></section>}
        {permissions.includes('companies.manage') && <section className="company-list-section"><div className="section-heading"><div><span className="panel-kicker">Tenant directory</span><h2>Companies</h2></div><button className="quiet-button" onClick={() => refreshCompanies()} disabled={companiesLoading}>{companiesLoading ? 'Refreshing...' : 'Refresh list'}</button></div><div className="company-stats"><div><strong>{companies.length}</strong><span>Total companies</span></div><div><strong>{activeCompanyCount}</strong><span>Active</span></div><div><strong>{suspendedCompanyCount}</strong><span>Suspended</span></div></div>{managementError && <div className="form-error" role="alert">{managementError}</div>}<div className="directory-toolbar"><label htmlFor="company-filter">Show<select id="company-filter" value={companyFilter} onChange={(event) => setCompanyFilter(event.target.value as typeof companyFilter)}><option value="all">All companies</option><option value="active">Active only</option><option value="suspended">Suspended only</option></select></label><span className="company-count">{filteredCompanies.length} shown</span></div><div className="company-list">{companiesLoading ? <div className="directory-message">Loading companies...</div> : filteredCompanies.length === 0 ? <div className="directory-message">No companies match this filter.</div> : filteredCompanies.map((item) => <article className="company-row" key={item.id}><div><strong>{item.name}</strong><span>{item.slug}</span><span className="lifecycle-time">Suspended: {formatLifecycleTime(item.suspended_at)}</span><span className="lifecycle-time">Activated: {formatLifecycleTime(item.activated_at)}</span></div><span className={`status-badge ${item.status}`}>{item.status}</span><div className="company-actions"><button className="quiet-button" onClick={() => beginEdit(item)}>Edit</button>{item.status === 'active' ? <button className="danger-button" onClick={() => suspendCompany(item.id)} disabled={suspendingCompanyId === item.id}>{suspendingCompanyId === item.id ? 'Suspending...' : 'Suspend'}</button> : <button className="primary-button compact-button" onClick={() => reactivateCompany(item.id)} disabled={suspendingCompanyId === item.id}>{suspendingCompanyId === item.id ? 'Activating...' : 'Reactivate'}</button>}</div></article>)}</div></section>}
        {editingCompany && <section className="login-panel onboarding-panel"><div className="panel-heading"><span className="panel-kicker">Company details</span><h2>Edit {editingCompany.name}</h2></div><form onSubmit={updateCompany}><label>Company name<input value={editName} onChange={(event) => setEditName(event.target.value)} required /></label><label>Company slug<input value={editSlug} onChange={(event) => setEditSlug(event.target.value)} pattern="[a-z0-9]+(?:-[a-z0-9]+)*" required /></label><div className="company-actions"><button className="primary-button" type="submit">Save changes</button><button className="quiet-button" type="button" onClick={() => setEditingCompany(null)}>Cancel</button></div></form></section>}
      </section>
    </main>
  );
}

function PlatformApp() {
  const [user, setUser] = useState<PlatformUser | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    platformRequest<PlatformSession>('/auth/refresh', { method: 'POST' })
      .then((session) => { platformAccessToken = session.access_token; if (active) { setUser(session.user); setPermissions(session.permissions); } })
      .catch(() => { platformAccessToken = ''; })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  async function logout() {
    await platformRequest('/auth/logout', { method: 'POST' }).catch(() => undefined);
    platformAccessToken = '';
    setUser(null);
    setPermissions([]);
  }

  if (loading) return <main className="loading-screen"><span className="loading-dot" />Restoring platform session...</main>;
  if (!user) return <PlatformLogin onLogin={(session) => { setUser(session.user); setPermissions(session.permissions); }} />;
  return <PlatformWorkspace user={user} permissions={permissions} onLogout={logout} />;
}

export default function App() {
  if (window.location.pathname.startsWith('/platform')) return <PlatformApp />;
  const [user, setUser] = useState<User | null>(null);
  const [permissions, setPermissions] = useState<Permissions>({ company: [], warehouses: {} });
  const [loading, setLoading] = useState(true);
  const [showWarehouses, setShowWarehouses] = useState(false);
  const [showPeople, setShowPeople] = useState(false);

  useEffect(() => {
    let active = true;
    request<SessionResponse>('/auth/refresh', { method: 'POST' })
      .then((session) => {
        accessToken = session.access_token;
        return request<MeResponse>('/me');
      })
      .then((me) => {
        if (active) {
          setUser(me.user);
          setPermissions(me.permissions);
        }
      })
      .catch(() => {
        accessToken = '';
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  async function logout() {
    await request('/auth/logout', { method: 'POST' }).catch(() => undefined);
    accessToken = '';
    setUser(null);
    setPermissions({ company: [], warehouses: {} });
  }

  if (loading) {
    return <main className="loading-screen"><span className="loading-dot" />Restoring your session...</main>;
  }
  if (!user) {
    return <LoginView onLogin={(session) => { setUser(session.user); setPermissions(session.permissions); }} />;
  }
  if (showWarehouses) return <WarehouseDashboard canManage={permissions.company.includes('warehouses.manage')} onBack={() => setShowWarehouses(false)} />;
  if (showPeople) return <PeopleDashboard user={user} permissions={permissions} onBack={() => setShowPeople(false)} />;
  return <Workspace user={user} permissions={permissions} onLogout={logout} onOpenWarehouses={() => setShowWarehouses(true)} onOpenPeople={() => setShowPeople(true)} />;
}
