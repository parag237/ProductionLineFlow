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
  const [filterWarehouseID, setFilterWarehouseID] = useState('all');
  const [entryWarehouseID, setEntryWarehouseID] = useState('');
  const [filterDate, setFilterDate] = useState(localDate);
  const [entryDate, setEntryDate] = useState(localDate);
  const [items, setItems] = useState<OperationLogItemOption[]>([]);
  const [performers, setPerformers] = useState<OperationLogPerformerOption[]>([]);
  const [entries, setEntries] = useState<OperationLogEntry[]>([]);
  const [editingEntry, setEditingEntry] = useState<OperationLogEntry | null>(null);
  const [itemID, setItemID] = useState('');
  const [stepID, setStepID] = useState('');
  const [performerID, setPerformerID] = useState('');
  const [quantity, setQuantity] = useState('1');
  const [loadingWarehouses, setLoadingWarehouses] = useState(true);
  const [loadingOptions, setLoadingOptions] = useState(false);
  const [loadingEntries, setLoadingEntries] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const canViewAny = permissions.company.includes('operations.logs.view') || permissions.company.includes('operations.logs.edit') || Object.values(permissions.warehouses).some((list) => list.includes('operations.logs.view') || list.includes('operations.logs.edit'));
  const canEditWorkDate = permissions.company.includes('operations.logs.date_override') || permissions.company.includes('admins.manage');
  const selectedFilterWarehouseID = Number(filterWarehouseID);
  const selectedEntryWarehouseID = Number(entryWarehouseID);
  const canViewFilter = filterWarehouseID === 'all' ? canViewAny : Number.isInteger(selectedFilterWarehouseID) && selectedFilterWarehouseID > 0 && (hasPermission(permissions, 'operations.logs.view', selectedFilterWarehouseID) || hasPermission(permissions, 'operations.logs.edit', selectedFilterWarehouseID));
  const canCreateSelected = Number.isInteger(selectedEntryWarehouseID) && selectedEntryWarehouseID > 0 && hasPermission(permissions, 'operations.logs.create', selectedEntryWarehouseID);
  const canEditSelected = editingEntry !== null && editingEntry.warehouse_id === selectedEntryWarehouseID && hasPermission(permissions, 'operations.logs.edit', editingEntry.warehouse_id);
  const visibleWarehouses = warehouses.filter((warehouse) => hasPermission(permissions, 'operations.logs.view', warehouse.id) || hasPermission(permissions, 'operations.logs.edit', warehouse.id));
  const creatableWarehouses = warehouses.filter((warehouse) => hasPermission(permissions, 'operations.logs.create', warehouse.id));
  const entryWarehouses = editingEntry ? warehouses.filter((warehouse) => warehouse.id === editingEntry.warehouse_id) : creatableWarehouses;
  const selectedItem = items.find((item) => item.id === Number(itemID));
  const displayDate = filterDate ? new Date(`${filterDate}T12:00:00`).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' }) : 'Select a work date';

  useEffect(() => {
    let active = true;
    request<OperationLogOptions>('/operation-logs/options')
      .then((options) => {
        if (!active) return;
        const availableWarehouses = options.warehouses ?? [];
        setWarehouses(availableWarehouses);
        const firstAllowed = availableWarehouses.find((warehouse) => hasPermission(permissions, 'operations.logs.create', warehouse.id) || hasPermission(permissions, 'operations.logs.edit', warehouse.id));
        setEntryWarehouseID((current) => current || String(firstAllowed?.id ?? ''));
        setFilterWarehouseID(canViewAny ? 'all' : '');
        const currentDate = options.today || localDate();
        setFilterDate(currentDate);
        setEntryDate(currentDate);
      })
      .catch((loadError) => {
        if (active) setError((loadError as ApiError).message);
      })
      .finally(() => {
        if (active) setLoadingWarehouses(false);
      });
    return () => { active = false; };
  }, [canViewAny, permissions]);

  useEffect(() => {
    if (!entryWarehouseID || (!canCreateSelected && !canEditSelected)) {
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
    request<OperationLogOptions>(`/operation-logs/options?warehouse_id=${encodeURIComponent(entryWarehouseID)}`)
      .then((options) => {
        if (!active) return;
        setItems(options.items ?? []);
        setPerformers(options.performers ?? []);
        if (!editingEntry || editingEntry.warehouse_id !== Number(entryWarehouseID)) {
          setItemID('');
          setStepID('');
          setPerformerID('');
        }
      })
      .catch((loadError) => {
        if (active) setError((loadError as ApiError).message);
      })
      .finally(() => {
        if (active) setLoadingOptions(false);
      });
    return () => { active = false; };
  }, [entryWarehouseID, canCreateSelected, canEditSelected, editingEntry?.id]);

  useEffect(() => {
    if (!filterDate || !canViewFilter) {
      setEntries([]);
      setLoadingEntries(false);
      return;
    }
    let active = true;
    setLoadingEntries(true);
    setError('');
    const query = new URLSearchParams({ warehouse_id: filterWarehouseID || 'all', work_date: filterDate });
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
  }, [filterWarehouseID, filterDate, canViewFilter]);

  async function saveEntry(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const isEditing = editingEntry !== null;
    if (!entryDate || !entryWarehouseID) {
      setError('Select a work date.');
      return;
    }
    const quantityValue = Number(quantity);
    if (!Number.isFinite(quantityValue) || quantityValue <= 0) {
      setError('Enter a quantity greater than zero.');
      return;
    }
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      const entry = await request<OperationLogEntry>(editingEntry ? `/operation-logs/${editingEntry.id}` : '/operation-logs', {
        method: editingEntry ? 'PATCH' : 'POST',
        body: JSON.stringify({
          warehouse_id: selectedEntryWarehouseID,
          work_date: entryDate,
          item_id: Number(itemID),
          step_id: Number(stepID),
          quantity: quantityValue,
          performed_by: Number(performerID),
        }),
      });
      if (isEditing) {
        setEntries((current) => {
          const remaining = current.filter((currentEntry) => currentEntry.id !== entry.id);
          return entry.work_date === filterDate && (filterWarehouseID === 'all' || entry.warehouse_id === selectedFilterWarehouseID) ? [entry, ...remaining] : remaining;
        });
        setEditingEntry(null);
      } else if (canViewFilter && entry.work_date === filterDate && (filterWarehouseID === 'all' || entry.warehouse_id === selectedFilterWarehouseID)) {
        setEntries((current) => [entry, ...current]);
      }
      setQuantity('1');
      setSuccess(isEditing ? 'Work entry updated.' : 'Work entry recorded.');
    } catch (saveError) {
      setError((saveError as ApiError).message);
    } finally {
      setSaving(false);
    }
  }

  function startEditEntry(entry: OperationLogEntry) {
    setEditingEntry(entry);
    setEntryWarehouseID(String(entry.warehouse_id));
    setEntryDate(entry.work_date);
    setItemID(String(entry.item_id));
    setStepID(String(entry.step_id));
    setPerformerID(String(entry.performed_by));
    setQuantity(String(entry.quantity));
    setError('');
    setSuccess('');
  }

  function cancelEditEntry() {
    setEditingEntry(null);
    setEntryWarehouseID(String(creatableWarehouses[0]?.id ?? ''));
    setEntryDate(filterDate || localDate());
    setItemID('');
    setStepID('');
    setPerformerID('');
    setQuantity('1');
    setSuccess('');
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
          {canViewAny && <div className="operation-log-filters">
            <label>Warehouse<select value={filterWarehouseID} onChange={(event) => setFilterWarehouseID(event.target.value)} disabled={loadingWarehouses || visibleWarehouses.length === 0}>
              <option value="all">All warehouses</option>
              {visibleWarehouses.map((warehouse) => <option key={warehouse.id} value={warehouse.id}>{warehouse.name}</option>)}
            </select></label>
            <label>Work date<input type="date" value={filterDate} onChange={(event) => setFilterDate(event.target.value)} required /></label>
          </div>}
        </div>

        {error && <div className="form-error" role="alert">{error}</div>}
        {loadingWarehouses ? <div className="directory-message">Loading warehouses...</div> : warehouses.length === 0 ? <div className="empty-state">No warehouses are available for your operations permissions.</div> : (
          <div className={`operation-log-layout ${(canCreateSelected || canEditSelected) && canViewFilter ? '' : 'single-column'}`}>
            {(canCreateSelected || canEditSelected) && <section className="operation-log-form-panel">
              <div className="operation-log-panel-heading">
                <span className="panel-kicker">New entry</span>
                <h2>{editingEntry ? 'Edit work entry' : 'Log Today\'s Entry'}</h2>
              </div>
              <form onSubmit={saveEntry} onInvalid={markFormSubmitted}>
                <label>Warehouse<select value={entryWarehouseID} onChange={(event) => setEntryWarehouseID(event.target.value)} required disabled={Boolean(editingEntry) || entryWarehouses.length === 0}>
                  <option value="">Select a warehouse</option>
                  {entryWarehouses.map((warehouse) => <option key={warehouse.id} value={warehouse.id}>{warehouse.name}</option>)}
                </select></label>
                <label>Work date<input type="date" value={entryDate} onChange={(event) => setEntryDate(event.target.value)} required disabled={!canEditWorkDate} /></label>
                <label>Item<select value={itemID} onChange={(event) => { setItemID(event.target.value); setStepID(''); }} required disabled={loadingOptions || items.length === 0}>
                  <option value="">Select an item</option>
                  {items.map((item) => <option key={item.id} value={item.id}>{item.name} · {item.unit_of_measure}</option>)}
                </select></label>
                <label>Step<select value={stepID} onChange={(event) => setStepID(event.target.value)} required disabled={!selectedItem || selectedItem.steps.length === 0}>
                  <option value="">Select a step</option>
                  {selectedItem?.steps.map((step) => <option key={step.id} value={step.id}>{step.title}</option>)}
                </select></label>
                <label>Quantity{selectedItem ? ` (${selectedItem.unit_of_measure})` : ''}<input type="number" min="0" step="any" value={quantity} onChange={(event) => setQuantity(event.target.value)} required /></label>
                <label>Performed by<select value={performerID} onChange={(event) => setPerformerID(event.target.value)} required disabled={loadingOptions || performers.length === 0}>
                  <option value="">Select a worker</option>
                  {performers.map((performer) => <option key={performer.id} value={performer.id}>{performer.name}</option>)}
                </select></label>
                {success && <div className="operation-log-success" role="status">{success}</div>}
                {items.length === 0 && !loadingOptions && <div className="operation-log-note">No active items with steps are available.</div>}
                {performers.length === 0 && !loadingOptions && <div className="operation-log-note">No active workers are assigned to this warehouse.</div>}
                <div className="company-actions"><button className="primary-button compact-button" type="submit" disabled={saving || loadingOptions || items.length === 0 || performers.length === 0}>{saving ? 'Saving...' : editingEntry ? 'Save changes' : 'Record work'}</button>{editingEntry && <button className="quiet-button" type="button" disabled={saving} onClick={cancelEditEntry}>Cancel edit</button>}</div>
              </form>
            </section>}

            {canViewFilter && <section className="operation-log-list-panel">
              <div className="operation-log-panel-heading">
                <div><span className="panel-kicker">Daily entries</span><h2>{displayDate}</h2></div>
                <span className="operation-log-count">{entries.length} {entries.length === 1 ? 'entry' : 'entries'}</span>
              </div>
              {loadingEntries ? <div className="directory-message">Loading entries...</div> : entries.length === 0 ? <div className="operation-log-empty">No work recorded for this date.</div> : (
                <div className="operation-log-table-wrap"><table className="operation-log-table">
                  <thead><tr>{filterWarehouseID === 'all' && <th>Warehouse</th>}<th>Item / step</th><th>Quantity</th><th>Performed by</th><th>Recorded</th><th>Actions</th></tr></thead>
                  <tbody>{entries.map((entry) => <tr key={entry.id}>
                    {filterWarehouseID === 'all' && <td>{entry.warehouse_name}</td>}
                    <td><strong>{entry.item_name}</strong><span>{entry.step_title}</span></td>
                    <td className="operation-log-quantity">{entry.quantity.toLocaleString(undefined, { maximumFractionDigits: 12 })} {entry.unit_of_measure}</td>
                    <td>{entry.performer_name}</td>
                    <td>{new Date(entry.created_at).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}</td>
                    <td>{hasPermission(permissions, 'operations.logs.edit', entry.warehouse_id) && <button className="quiet-button compact-button" type="button" onClick={() => startEditEntry(entry)}>Edit</button>}</td>
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