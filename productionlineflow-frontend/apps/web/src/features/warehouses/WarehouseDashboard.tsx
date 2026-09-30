import { useEffect, useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import { markFormSubmitted } from '../../shared/forms';
import type { ApiError, Warehouse } from '../../types';

export default function WarehouseDashboard({ canManage }: { canManage: boolean }) {
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

  return <main className="workspace-shell"><section className="workspace-content"><div className="workspace-heading"><div><span className="panel-kicker">Company warehouses</span><h1>Warehouse dashboard.</h1><p>Manage every operating location in this company.</p></div>{canManage && <button className="primary-button compact-button" onClick={startCreate}>Add warehouse</button>}</div>{error && <div className="form-error" role="alert">{error}</div>}{showForm && <section className="login-panel onboarding-panel"><div className="panel-heading"><span className="panel-kicker">{editing ? 'Edit location' : 'New location'}</span><h2>{editing ? 'Update warehouse' : 'Add warehouse'}</h2></div><form onSubmit={saveWarehouse} onInvalid={markFormSubmitted}><label>Warehouse name<input value={name} onChange={(event) => setName(event.target.value)} required /></label><label>Warehouse type<input value={typeName} onChange={(event) => setTypeName(event.target.value)} required placeholder="Factory" /></label><label>Address<input value={address} onChange={(event) => setAddress(event.target.value)} /></label><div className="company-actions"><button className="primary-button compact-button" type="submit">{editing ? 'Save changes' : 'Create warehouse'}</button><button className="quiet-button" type="button" onClick={() => setShowForm(false)}>Cancel</button></div></form></section>}<div className="warehouse-list">{loading ? <div className="directory-message">Loading warehouses...</div> : warehouses.length === 0 ? <div className="empty-state">No warehouses yet. Add the first operating location.</div> : warehouses.map((item) => <article className="warehouse-row" key={item.id}><div><strong>{item.name}</strong><span>{item.type_name} · {item.address || 'No address'}</span></div><span className="status-badge active">{item.state}</span>{canManage && <div className="company-actions"><button className="quiet-button" onClick={() => startEdit(item)}>Edit</button><button className="danger-button" onClick={() => deleteWarehouse(item.id)}>Delete</button></div>}</article>)}</div></section></main>;
}
