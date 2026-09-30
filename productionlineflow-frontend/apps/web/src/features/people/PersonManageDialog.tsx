import { useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import type { ApiError, Assignment, Person, Role, Warehouse } from '../../types';

type Props = {
  person: Person;
  roles: Role[];
  warehouses: Warehouse[];
  canManageCompany: boolean;
  canManageAdmins: boolean;
  managedWarehouseIds: number[];
  onClose: () => void;
  onTransferred: () => Promise<void>;
  onUpdated: () => Promise<void>;
};

export default function PersonManageDialog({ person, roles, warehouses, canManageCompany, canManageAdmins, managedWarehouseIds, onClose, onTransferred, onUpdated }: Props) {
  const [tab, setTab] = useState<'role' | 'password' | 'edit'>('role');
  const [current, setCurrent] = useState(person);
  const [roleID, setRoleID] = useState('');
  const [warehouseID, setWarehouseID] = useState('');
  const [name, setName] = useState(person.name);
  const [email, setEmail] = useState(person.email);
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [saving, setSaving] = useState(false);
  const allowedRoles = roles.filter((role) => role.slug !== 'super_admin' && (canManageAdmins || role.scope === 'warehouse') && (canManageCompany || role.slug === 'worker'));
  const availableWarehouses = warehouses.filter((item) => canManageCompany || managedWarehouseIds.includes(item.id));
  const privileged = current.assignments.some((assignment) => ['admin', 'super_admin'].includes(assignment.role_slug));
  const canEdit = canManageCompany && (!privileged || canManageAdmins);
  const canPassword = canManageCompany && (!privileged || canManageAdmins);
  const canTransfer = canManageAdmins && current.is_active && !current.assignments.some((assignment) => assignment.role_slug === 'super_admin');

  async function run(action: () => Promise<void>, success?: string) {
    setError(''); setMessage(''); setSaving(true);
    try { await action(); if (success) setMessage(success); await onUpdated(); }
    catch (e) { setError((e as ApiError).message); }
    finally { setSaving(false); }
  }
  async function addAssignment() {
    const role = roles.find((item) => item.id === Number(roleID));
    if (!role) return;
    await run(async () => {
      await request(`/users/${current.id}/role-assignments`, { method: 'POST', body: JSON.stringify({ role_id: role.id, warehouse_id: role.scope === 'warehouse' ? Number(warehouseID) : null }) });
      setRoleID(''); setWarehouseID('');
      const refreshed = await request<Person>(`/users/${current.id}`); setCurrent(refreshed);
    });
  }
  async function removeAssignment(assignment: Assignment) {
    await run(async () => { await request(`/users/${current.id}/role-assignments/${assignment.id}`, { method: 'DELETE' }); setCurrent(await request<Person>(`/users/${current.id}`)); });
  }
  async function saveEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await run(async () => { await request(`/users/${current.id}`, { method: 'PATCH', body: JSON.stringify({ name, email }) }); setCurrent(await request<Person>(`/users/${current.id}`)); }, 'Person details updated.');
  }
  async function resetPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (password !== confirmation) { setError('The passwords do not match.'); return; }
    await run(async () => { await request(`/users/${current.id}/password`, { method: 'PATCH', body: JSON.stringify({ password }) }); setPassword(''); setConfirmation(''); }, 'Password reset successfully.');
  }
  async function transfer() {
    if (!window.confirm(`Transfer Super Admin to ${current.name}?`)) return;
    setError(''); setSaving(true);
    try { await request(`/users/${current.id}/transfer-super-admin`, { method: 'POST' }); await onTransferred(); }
    catch (e) { setError((e as ApiError).message); setSaving(false); }
  }
  const canManageAnyRole = allowedRoles.length > 0 || canManageAdmins || current.assignments.some((a) => canManageAdmins || (a.role_slug === 'worker' && managedWarehouseIds.includes(a.warehouse_id ?? -1)));
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget && !saving) onClose(); }}><section className="people-dialog manage-person-dialog" role="dialog" aria-modal="true" aria-labelledby="manage-person-title">
    <div className="dialog-heading"><div><span className="panel-kicker">Person settings</span><h2 id="manage-person-title">Manage person</h2><p>{current.name} · {current.email}</p></div><button className="dialog-close" aria-label="Close manage person" disabled={saving} onClick={onClose}>×</button></div>
    <div className="manage-dialog-tabs" role="tablist" aria-label="Person settings">{canManageAnyRole && <button role="tab" aria-selected={tab === 'role'} onClick={() => { setTab('role'); setError(''); setMessage(''); }}>Role</button>}{canPassword && <button role="tab" aria-selected={tab === 'password'} onClick={() => { setTab('password'); setError(''); setMessage(''); }}>Password</button>}{canEdit && <button role="tab" aria-selected={tab === 'edit'} onClick={() => { setTab('edit'); setError(''); setMessage(''); }}>Edit</button>}</div>
    {tab === 'role' && canManageAnyRole && <div className="dialog-section"><h3>Current roles</h3>{current.assignments.length === 0 ? <p className="muted-role">No roles assigned yet.</p> : <ul className="dialog-role-list">{current.assignments.map((assignment) => <li key={assignment.id}><span><strong>{assignment.role_name}</strong>{assignment.warehouse_name && <small>{assignment.warehouse_name}</small>}</span>{assignment.role_slug !== 'super_admin' && (canManageAdmins || (!canManageCompany && assignment.role_slug === 'worker' && managedWarehouseIds.includes(assignment.warehouse_id ?? -1))) && <button className="quiet-button" disabled={saving} onClick={() => void removeAssignment(assignment)}>Remove</button>}</li>)}</ul>}{allowedRoles.length > 0 && current.is_active && <><h3>Assign a role</h3><div className="role-assignment-form"><label>Role<select value={roleID} onChange={(event) => { setRoleID(event.target.value); setWarehouseID(''); }}><option value="">Choose a role</option>{allowedRoles.map((role) => <option key={role.id} value={role.id}>{role.name} · {role.scope}</option>)}</select></label>{roleID && roles.find((role) => role.id === Number(roleID))?.scope === 'warehouse' && <label>Warehouse<select value={warehouseID} onChange={(event) => setWarehouseID(event.target.value)}><option value="">Choose a warehouse</option>{availableWarehouses.map((warehouse) => <option key={warehouse.id} value={warehouse.id}>{warehouse.name}</option>)}</select></label>}<button className="primary-button" disabled={saving || !roleID || (roles.find((role) => role.id === Number(roleID))?.scope === 'warehouse' && !warehouseID)} onClick={() => void addAssignment()}>Assign role</button></div></>}</div>}
    {tab === 'password' && canPassword && <form className="password-reset-form" onSubmit={resetPassword}><label>New Password<input type="password" autoComplete="new-password" minLength={10} required value={password} onChange={(event) => setPassword(event.target.value)} /></label><label>Confirm Password<input type="password" autoComplete="new-password" minLength={10} required value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label><small>Use at least 10 characters.</small><div className="dialog-footer"><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Resetting…' : 'Reset Password'}</button></div></form>}
    {tab === 'edit' && canEdit && <div className="dialog-section"><form className="person-edit-form" onSubmit={saveEdit}><label>Name<input required value={name} onChange={(event) => setName(event.target.value)} /></label><label>Company email<input type="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label><div className="dialog-footer"><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save changes'}</button></div></form>{canTransfer && <div className="transfer-super-admin"><h3>Transfer Super Admin</h3><p>Make this person the company Super Admin and demote the current one to Admin.</p><button className="quiet-button" disabled={saving} onClick={() => void transfer()}>Transfer Super Admin</button></div>}</div>}
    {error && <div className="form-error" role="alert">{error}</div>}{message && <div className="success-message" role="status">{message}</div>}
    <div className="dialog-footer"><button className="quiet-button" disabled={saving} onClick={onClose}>Done</button></div>
  </section></div>;
}
