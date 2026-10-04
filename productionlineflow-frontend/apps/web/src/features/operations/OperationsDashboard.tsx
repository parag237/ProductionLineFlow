import { useEffect, useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import { markFormSubmitted } from '../../shared/forms';
import type { ApiError, OperationLogEntry, OperationLogItemOption, OperationLogOptions, OperationLogPerformerOption, Permissions } from '../../types';

function localDate() {
  const today = new Date();
  return `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;
}

function hasPermission(permissions: Permissions, permission: string, warehouseID: number) {
  return permissions.company.includes(permission) || permissions.warehouses[String(warehouseID)]?.includes(permission) === true;
}

export default function OperationsDashboard({ permissions }: { permissions: Permissions }) {
  const [warehouses, setWarehouses] = useState<OperationLogOptions['warehouses']>([]);
  const [warehouseID, setWarehouseID] = useState('');
  const [workDate, setWorkDate] = useState(localDate);
  const [items, setItems] = useState<OperationLogItemOption[]>([]);
  const [performers, setPerformers] = useState<OperationLogPerformerOption[]>([]);
  const [entries, setEntries] = useState<OperationLogEntry[]>([]);
  const [itemID, setItemID] = useState('');
  const [stepID, setStepID] = useState('');
  const [performerID, setPerformerID] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [loadingWarehouses, setLoadingWarehouses] = useState(true);
  const [loadingOptions, setLoadingOptions] = useState(false);
  const [loadingEntries, setLoadingEntries] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const selectedWarehouseID = Number(warehouseID);
  const canViewSelected = Number.isInteger(selectedWarehouseID) && selectedWarehouseID > 0 && hasPermission(permissions, 'operations.logs.view', selectedWarehouseID);
  const canCreateSelected = Number.isInteger(selectedWarehouseID) && selectedWarehouseID > 0 && hasPermission(permissions, 'operations.logs.create', selectedWarehouseID);
  const selectedItem = items.find((item) => item.id === Number(itemID));
  const displayDate = workDate ? new Date(`${workDate}T12:00:00`).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' }) : 'Select a work date';

  useEffect(() => {
    let active = true;
    request<OperationLogOptions>('/operation-logs/options')
      .then((options) => {
        if (!active) return;
        const availableWarehouses = options.warehouses ?? [];
        setWarehouses(availableWarehouses);
        setWarehouseID((current) => current || String(availableWarehouses[0]?.id ?? ''));
      })
      .catch((loadError) => {
        if (active) setError((loadError as ApiError).message);
      })
      .finally(() => {
        if (active) setLoadingWarehouses(false);
      });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (!warehouseID || !canCreateSelected) {
      setItems([]);
      setPerformers([]);
      setItemID('');
      setStepID('');
      setPerformerID('');
      setLoadingOptions(false);
      return;
    }
    let active = true;
    setLoadingOptions(true);
    setError('');
    request<OperationLogOptions>(`/operation-logs/options?warehouse_id=${encodeURIComponent(warehouseID)}`)
      .then((options) => {
        if (!active) return;
        setItems(options.items ?? []);
        setPerformers(options.performers ?? []);
        setItemID('');
        setStepID('');
        setPerformerID('');
      })
      .catch((loadError) => {
        if (active) setError((loadError as ApiError).message);
      })
      .finally(() => {
        if (active) setLoadingOptions(false);
      });
    return () => { active = false; };
  }, [warehouseID, canCreateSelected]);

  useEffect(() => {
    if (!warehouseID || !workDate || !canViewSelected) {
      setEntries([]);
      setLoadingEntries(false);
      return;
    }
    let active = true;
    setLoadingEntries(true);
    setError('');
    const query = new URLSearchParams({ warehouse_id: warehouseID, work_date: workDate });
    request<{ entries: OperationLogEntry[] }>(`/operation-logs?${query.toString()}`)
      .then((result) => {
        if (active) setEntries(result.entries ?? []);
      })
      .catch((loadError) => {
        if (active) setError((loadError as ApiError).message);
      })
      .finally(() => {
        if (active) setLoadingEntries(false);
      });
    return () => { active = false; };
  }, [warehouseID, workDate, canViewSelected]);

  async function saveEntry(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!workDate) {
      setError('Select a work date.');
      return;
    }
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      const entry = await request<OperationLogEntry>('/operation-logs', {
        method: 'POST',
        body: JSON.stringify({
          warehouse_id: selectedWarehouseID,
          work_date: workDate,
          item_id: Number(itemID),
          step_id: Number(stepID),
          quantity,
          performed_by: Number(performerID),
        }),
      });
      if (canViewSelected && entry.work_date === workDate) setEntries((current) => [entry, ...current]);
      setQuantity(1);
      setSuccess('Work entry recorded.');
    } catch (saveError) {
      setError((saveError as ApiError).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <main className="workspace-shell">
      <section className="workspace-content operations-workspace">
        <div className="workspace-heading">
          <div>
            <span className="panel-kicker">Operations log</span>
            <h1>Work, recorded.</h1>
            <p>Daily work entries by item, step, and person.</p>
          </div>
          <div className="operation-log-filters">
            <label>Warehouse<select value={warehouseID} onChange={(event) => setWarehouseID(event.target.value)} disabled={loadingWarehouses || warehouses.length === 0} required>
              <option value="">Select a warehouse</option>
              {warehouses.map((warehouse) => <option key={warehouse.id} value={warehouse.id}>{warehouse.name}</option>)}
            </select></label>
            <label>Work date<input type="date" value={workDate} onChange={(event) => setWorkDate(event.target.value)} required /></label>
          </div>
        </div>

        {error && <div className="form-error" role="alert">{error}</div>}
        {loadingWarehouses ? <div className="directory-message">Loading warehouses...</div> : warehouses.length === 0 ? <div className="empty-state">No warehouses are available for your operations permissions.</div> : (
          <div className={`operation-log-layout ${canCreateSelected && canViewSelected ? '' : 'single-column'}`}>
            {canCreateSelected && <section className="operation-log-form-panel">
              <div className="operation-log-panel-heading">
                <span className="panel-kicker">New entry</span>
                <h2>Log Today&apos;s Entry</h2>
              </div>
              <form onSubmit={saveEntry} onInvalid={markFormSubmitted}>
                <label>Item<select value={itemID} onChange={(event) => { setItemID(event.target.value); setStepID(''); }} required disabled={loadingOptions || items.length === 0}>
                  <option value="">Select an item</option>
                  {items.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
                </select></label>
                <label>Step<select value={stepID} onChange={(event) => setStepID(event.target.value)} required disabled={!selectedItem || selectedItem.steps.length === 0}>
                  <option value="">Select a step</option>
                  {selectedItem?.steps.map((step) => <option key={step.id} value={step.id}>{step.title}</option>)}
                </select></label>
                <label>Units completed<input type="number" min={1} step={1} value={quantity} onChange={(event) => setQuantity(Number(event.target.value))} required /></label>
                <label>Performed by<select value={performerID} onChange={(event) => setPerformerID(event.target.value)} required disabled={loadingOptions || performers.length === 0}>
                  <option value="">Select a worker</option>
                  {performers.map((performer) => <option key={performer.id} value={performer.id}>{performer.name}</option>)}
                </select></label>
                {success && <div className="operation-log-success" role="status">{success}</div>}
                {items.length === 0 && !loadingOptions && <div className="operation-log-note">No active items with steps are available.</div>}
                {performers.length === 0 && !loadingOptions && <div className="operation-log-note">No active workers are assigned to this warehouse.</div>}
                <button className="primary-button compact-button" type="submit" disabled={saving || loadingOptions || items.length === 0 || performers.length === 0}>{saving ? 'Recording...' : 'Record work'}</button>
              </form>
            </section>}

            {canViewSelected && <section className="operation-log-list-panel">
              <div className="operation-log-panel-heading">
                <div><span className="panel-kicker">Daily entries</span><h2>{displayDate}</h2></div>
                <span className="operation-log-count">{entries.length} {entries.length === 1 ? 'entry' : 'entries'}</span>
              </div>
              {loadingEntries ? <div className="directory-message">Loading entries...</div> : entries.length === 0 ? <div className="operation-log-empty">No work recorded for this date.</div> : (
                <div className="operation-log-table-wrap"><table className="operation-log-table">
                  <thead><tr><th>Item / step</th><th>Units</th><th>Performed by</th><th>Recorded</th></tr></thead>
                  <tbody>{entries.map((entry) => <tr key={entry.id}>
                    <td><strong>{entry.item_name}</strong><span>{entry.step_title}</span></td>
                    <td className="operation-log-quantity">{entry.quantity.toLocaleString()}</td>
                    <td>{entry.performer_name}</td>
                    <td>{new Date(entry.created_at).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}</td>
                  </tr>)}</tbody>
                </table></div>
              )}
            </section>}
          </div>
        )}
      </section>
    </main>
  );
}