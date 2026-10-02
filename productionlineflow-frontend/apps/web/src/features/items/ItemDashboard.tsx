import { useEffect, useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import { markFormSubmitted } from '../../shared/forms';
import type { ApiError, Item, ItemStepInput, Permissions, ProductionRun, ProductionUnit } from '../../types';

type StepDraft = ItemStepInput & { key: number };

export default function ItemDashboard({ permissions }: { permissions: Permissions }) {
  const canManageItems = permissions.company.includes('items.manage');
  const canViewItems = permissions.company.includes('items.view') || canManageItems || permissions.company.includes('production.execute');
  const canViewProduction = permissions.company.includes('production.view') || permissions.company.includes('production.execute');
  const canExecuteProduction = permissions.company.includes('production.execute');
  const [view, setView] = useState<'catalog' | 'production'>(canViewItems ? 'catalog' : 'production');
  const [items, setItems] = useState<Item[]>([]);
  const [runs, setRuns] = useState<ProductionRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<Item | null>(null);
  const [showItemForm, setShowItemForm] = useState(false);
  const [name, setName] = useState('');
  const [sku, setSku] = useState('');
  const [description, setDescription] = useState('');
  const [steps, setSteps] = useState<StepDraft[]>([]);
  const [nextStepKey, setNextStepKey] = useState(1);
  const [selectedItemID, setSelectedItemID] = useState('');
  const [trackingMode, setTrackingMode] = useState<'batch' | 'unit'>('batch');
  const [quantity, setQuantity] = useState(1);
  const [serialNumbers, setSerialNumbers] = useState('');

  async function loadItems() {
    if (!canViewItems) return;
    const result = await request<{ items: Item[] }>('/items');
    setItems(result.items ?? []);
  }

  async function loadRuns() {
    if (!canViewProduction) return;
    const result = await request<{ runs: ProductionRun[] }>('/production/runs');
    setRuns(result.runs ?? []);
  }

  async function loadDashboard() {
    setLoading(true);
    setError('');
    try {
      await Promise.all([loadItems(), loadRuns()]);
    } catch (loadError) {
      setError((loadError as ApiError).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void loadDashboard(); }, [canViewItems, canViewProduction]);

  function startCreate() {
    const firstKey = nextStepKey;
    setNextStepKey(firstKey + 1);
    setEditing(null);
    setName('');
    setSku('');
    setDescription('');
    setSteps([{ key: firstKey, title: '', instructions: '' }]);
    setError('');
    setShowItemForm(true);
  }

  function startEdit(item: Item) {
    setEditing(item);
    setName(item.name);
    setSku(item.sku ?? '');
    setDescription(item.description ?? '');
    setSteps(item.steps.map((step) => ({ key: step.id, title: step.title, instructions: step.instructions ?? '' })));
    setError('');
    setShowItemForm(true);
  }

  function addStep() {
    setSteps((current) => [...current, { key: nextStepKey, title: '', instructions: '' }]);
    setNextStepKey((current) => current + 1);
  }

  function updateStep(key: number, field: keyof ItemStepInput, value: string) {
    setSteps((current) => current.map((step) => step.key === key ? { ...step, [field]: value } : step));
  }

  function moveStep(index: number, offset: -1 | 1) {
    setSteps((current) => {
      const destination = index + offset;
      if (destination < 0 || destination >= current.length) return current;
      const reordered = [...current];
      [reordered[index], reordered[destination]] = [reordered[destination], reordered[index]];
      return reordered;
    });
  }

  async function saveItem(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      const payload = JSON.stringify({ name, sku, description, steps: steps.map(({ title, instructions }) => ({ title, instructions })) });
      if (editing) await request(`/items/${editing.id}`, { method: 'PATCH', body: payload });
      else await request('/items', { method: 'POST', body: payload });
      setShowItemForm(false);
      await loadItems();
    } catch (saveError) {
      setError((saveError as ApiError).message);
    } finally {
      setSaving(false);
    }
  }

  async function archiveItem(item: Item) {
    if (!window.confirm(`Archive ${item.name}?`)) return;
    setError('');
    try {
      await request(`/items/${item.id}`, { method: 'DELETE' });
      await loadItems();
    } catch (archiveError) {
      setError((archiveError as ApiError).message);
    }
  }

  async function startProduction(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      const serialList = trackingMode === 'unit' ? serialNumbers.split(/\r?\n/).map((serial) => serial.trim()).filter(Boolean) : [];
      const run = await request<ProductionRun>('/production/runs', {
        method: 'POST',
        body: JSON.stringify({ item_id: Number(selectedItemID), tracking_mode: trackingMode, quantity: trackingMode === 'unit' ? serialList.length : quantity, serial_numbers: serialList }),
      });
      setRuns((current) => [run, ...current]);
      setSerialNumbers('');
      setQuantity(1);
    } catch (startError) {
      setError((startError as ApiError).message);
    } finally {
      setSaving(false);
    }
  }

  async function completeStep(run: ProductionRun, stepID: number, unit?: ProductionUnit) {
    setError('');
    try {
      const updated = await request<ProductionRun>(`/production/runs/${run.id}/steps/${stepID}/complete`, {
        method: 'POST',
        body: JSON.stringify({ unit_id: unit?.id ?? null }),
      });
      setRuns((current) => current.map((item) => item.id === updated.id ? updated : item));
    } catch (completeError) {
      setError((completeError as ApiError).message);
    }
  }

  function nextStepFor(run: ProductionRun, unit?: ProductionUnit) {
    return run.steps.find((step) => !run.events.some((event) => event.run_step_id === step.id && event.event === 'completed' && (unit ? event.unit_id === unit.id : event.unit_id == null)));
  }

  const activeItems = items.filter((item) => item.is_active);

  return (
    <main className="workspace-shell">
      <section className="workspace-content">
        <div className="workspace-heading">
          <div><span className="panel-kicker">Company production</span><h1>Items and production.</h1><p>Manage item steps and track production progress.</p></div>
          <div className="company-actions">
            {canViewItems && <button className={view === 'catalog' ? 'primary-button compact-button' : 'quiet-button'} onClick={() => setView('catalog')}>Items</button>}
            {canViewProduction && <button className={view === 'production' ? 'primary-button compact-button' : 'quiet-button'} onClick={() => setView('production')}>Production</button>}
          </div>
        </div>
        {error && <div className="form-error" role="alert">{error}</div>}

        {view === 'catalog' && canViewItems && <>
          <div className="workspace-heading"><div><span className="panel-kicker">Catalog</span><h2>Company items</h2></div>{canManageItems && <button className="primary-button compact-button" onClick={startCreate}>Add item</button>}</div>
          {showItemForm && <section className="login-panel onboarding-panel"><div className="panel-heading"><span className="panel-kicker">{editing ? 'Update item' : 'New item'}</span><h2>{editing ? 'Edit item' : 'Add item'}</h2></div><form onSubmit={saveItem} onInvalid={markFormSubmitted}>
            <label>Item name<input value={name} onChange={(event) => setName(event.target.value)} maxLength={120} required /></label>
            <label>SKU<input value={sku} onChange={(event) => setSku(event.target.value)} maxLength={64} /></label>
            <label>Description<textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} rows={3} /></label>
            <div className="panel-heading"><span className="panel-kicker">Production sequence</span><h3>Steps</h3></div>
            {steps.map((step, index) => <div className="warehouse-row" key={step.key}><div className="item-step-fields"><label>Step {index + 1}<input value={step.title} onChange={(event) => updateStep(step.key, 'title', event.target.value)} maxLength={120} required /></label><label>Instructions<textarea value={step.instructions} onChange={(event) => updateStep(step.key, 'instructions', event.target.value)} maxLength={2000} rows={2} /></label></div><div className="company-actions"><button className="quiet-button" type="button" disabled={index === 0} onClick={() => moveStep(index, -1)} aria-label={`Move step ${index + 1} up`}>Up</button><button className="quiet-button" type="button" disabled={index === steps.length - 1} onClick={() => moveStep(index, 1)} aria-label={`Move step ${index + 1} down`}>Down</button><button className="danger-button" type="button" disabled={steps.length === 1} onClick={() => setSteps((current) => current.filter((item) => item.key !== step.key))}>Remove</button></div></div>)}
            <div className="company-actions"><button className="quiet-button" type="button" onClick={addStep}>Add step</button><button className="primary-button compact-button" type="submit" disabled={saving}>{saving ? 'Saving...' : editing ? 'Save changes' : 'Create item'}</button><button className="quiet-button" type="button" onClick={() => setShowItemForm(false)}>Cancel</button></div>
          </form></section>}
          {loading ? <div className="directory-message">Loading items...</div> : items.length === 0 ? <div className="empty-state">No items yet. Add the first company item.</div> : <div className="warehouse-list">{items.map((item) => <article className="warehouse-row item-row" key={item.id}><div className="item-summary"><strong>{item.name}</strong><span>{item.sku || 'No SKU'}{item.description ? ` · ${item.description}` : ''}</span><ol>{item.steps.map((step) => <li key={step.id}>{step.title}</li>)}</ol></div><span className={`status-badge ${item.is_active ? 'active' : ''}`}>{item.is_active ? 'Active' : 'Archived'}</span>{canManageItems && item.is_active && <div className="company-actions"><button className="quiet-button" onClick={() => startEdit(item)}>Edit</button><button className="danger-button" onClick={() => void archiveItem(item)}>Archive</button></div>}</article>)}</div>}
        </>}

        {view === 'production' && canViewProduction && <>
          <div className="workspace-heading"><div><span className="panel-kicker">Production runs</span><h2>Work in progress</h2></div></div>
          {canExecuteProduction && <section className="login-panel onboarding-panel"><div className="panel-heading"><span className="panel-kicker">Start production</span><h2>New run</h2></div><form onSubmit={startProduction} onInvalid={markFormSubmitted}>
            <label>Item<select value={selectedItemID} onChange={(event) => setSelectedItemID(event.target.value)} required><option value="">Select an item</option>{activeItems.map((item) => <option key={item.id} value={item.id}>{item.name}{item.sku ? ` · ${item.sku}` : ''}</option>)}</select></label>
            <label>Tracking<select value={trackingMode} onChange={(event) => setTrackingMode(event.target.value as 'batch' | 'unit')}><option value="batch">Quantity batch</option><option value="unit">Individual units</option></select></label>
            {trackingMode === 'batch' ? <label>Quantity<input type="number" min={1} value={quantity} onChange={(event) => setQuantity(Number(event.target.value))} required /></label> : <label>Serial numbers, one per line<textarea value={serialNumbers} onChange={(event) => setSerialNumbers(event.target.value)} rows={4} required /></label>}
            <button className="primary-button compact-button" type="submit" disabled={saving || activeItems.length === 0}>{saving ? 'Starting...' : 'Start run'}</button>
          </form></section>}
          {loading ? <div className="directory-message">Loading production...</div> : runs.length === 0 ? <div className="empty-state">No production runs yet.</div> : <div className="warehouse-list">{runs.map((run) => (
            <article className="warehouse-row production-run-row" key={run.id}>
              <div className="production-run-heading">
                <div><strong>{run.item_name}</strong><span>Run {run.id} · {run.tracking_mode === 'batch' ? `${run.quantity} units in batch` : `${run.quantity} serialised units`}</span></div>
                <span className={`status-badge ${run.status === 'completed' ? 'active' : ''}`}>{run.status.replace('_', ' ')}</span>
              </div>
              {run.tracking_mode === 'batch' ? <ol className="production-steps">{run.steps.map((step) => {
                const completed = run.events.some((event) => event.run_step_id === step.id && event.event === 'completed' && event.unit_id == null);
                return <li key={step.id} className={completed ? 'step-complete' : ''}><span>{step.title}</span>{!completed && run.status === 'in_progress' && canExecuteProduction && nextStepFor(run)?.id === step.id && <button className="quiet-button" onClick={() => void completeStep(run, step.id)}>Complete step</button>}</li>;
              })}</ol> : <div className="production-units">{run.units.map((unit) => {
                const nextStep = nextStepFor(run, unit);
                return <div className="production-unit-row" key={unit.id}><span><strong>{unit.serial_number}</strong><small>{unit.status.replace('_', ' ')}</small></span>{nextStep && canExecuteProduction && run.status === 'in_progress' && <button className="quiet-button" onClick={() => void completeStep(run, nextStep.id, unit)}>Complete: {nextStep.title}</button>}</div>;
              })}</div>}
              <details className="production-history">
                <summary>Step history ({run.events.length})</summary>
                {run.events.length === 0 ? <p>No steps completed yet.</p> : <ol>{run.events.map((event) => {
                  const step = run.steps.find((candidate) => candidate.id === event.run_step_id);
                  const unit = run.units.find((candidate) => candidate.id === event.unit_id);
                  return <li key={event.id}><span>{unit ? `${unit.serial_number} · ` : 'Batch · '}{step?.title ?? 'Step'} · {event.event} · user {event.actor_id}</span><time dateTime={event.created_at}>{new Date(event.created_at).toLocaleString()}</time>{event.note && <p>{event.note}</p>}</li>;
                })}</ol>}
              </details>
            </article>
          ))}</div>}
        </>}
      </section>
    </main>
  );
}
