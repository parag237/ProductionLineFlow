import { useEffect, useState, type FormEvent } from 'react';
import { platformRequest } from '../../api/client';
import { markFormSubmitted } from '../../shared/forms';
import type { ApiError, PlatformCompany, PlatformUser } from '../../types';

export default function PlatformWorkspace({ user, permissions, onLogout }: { user: PlatformUser; permissions: string[]; onLogout: () => void }) {
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
