import { useEffect, useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import { markFormSubmitted } from '../../shared/forms';
import type { ApiError, Category, Item, ItemStepInput, Permissions } from '../../types';
import CategoryManager from './CategoryManager';

type StepDraft = ItemStepInput & { key: number };

const unitChoices = [
  { value: 'pieces', label: 'Pieces' },
  { value: 'kg', label: 'Kilograms (kg)' },
  { value: 'g', label: 'Grams (g)' },
  { value: 'tonne', label: 'Tonnes (t)' },
  { value: 'dozen', label: 'Dozen' },
  { value: 'litre', label: 'Litres (L)' },
  { value: 'ml', label: 'Millilitres (ml)' },
  { value: 'metre', label: 'Metres (m)' },
  { value: 'cm', label: 'Centimetres (cm)' },
  { value: 'box', label: 'Boxes' },
  { value: 'pack', label: 'Packs' },
  { value: 'set', label: 'Sets' },
  { value: 'roll', label: 'Rolls' },
  { value: 'bag', label: 'Bags' },
];

export default function CatalogDashboard({ permissions }: { permissions: Permissions }) {
  const canManageItems = permissions.company.includes('items.manage');
  const canViewItems = permissions.company.includes('items.view') || canManageItems;
  const canManageCategories = permissions.company.includes('items.categories.manage');
  const canViewCategories = canManageCategories || permissions.company.includes('items.categories.view');
  const [view, setView] = useState<'items' | 'categories'>(canViewItems ? 'items' : 'categories');
  const [items, setItems] = useState<Item[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<Item | null>(null);
  const [showItemForm, setShowItemForm] = useState(false);
  const [name, setName] = useState('');
  const [sku, setSku] = useState('');
  const [description, setDescription] = useState('');
  const [unitChoice, setUnitChoice] = useState('pieces');
  const [customUnit, setCustomUnit] = useState('');
  const [categoryID, setCategoryID] = useState('');
  const [steps, setSteps] = useState<StepDraft[]>([]);
  const [nextStepKey, setNextStepKey] = useState(1);

  async function loadItems() {
    if (!canViewItems) return;
    const result = await request<{ items: Item[] }>('/items');
    setItems(result.items ?? []);
  }

  async function loadCategories() {
    if (!canViewCategories) return;
    const result = await request<{ categories: Category[] }>('/categories');
    const available = result.categories ?? [];
    setCategories(available);
    setCategoryID((current) => current || String(available[0]?.id ?? ''));
  }

  async function loadDashboard() {
    setLoading(true);
    setError('');
    try {
      await Promise.all([loadItems(), loadCategories()]);
    } catch (loadError) {
      setError((loadError as ApiError).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void loadDashboard(); }, [canViewItems, canViewCategories]);

  function startCreate() {
    const firstKey = nextStepKey;
    setNextStepKey((current) => current + 1);
    setEditing(null);
    setName('');
    setSku('');
    setDescription('');
    setUnitChoice('pieces');
    setCustomUnit('');
    setCategoryID(String(categories[0]?.id ?? ''));
    setSteps([{ key: firstKey, title: '', instructions: '' }]);
    setError('');
    setShowItemForm(true);
  }

  function startEdit(item: Item) {
    setEditing(item);
    setName(item.name);
    setSku(item.sku ?? '');
    setDescription(item.description ?? '');
    const knownUnit = unitChoices.some((unit) => unit.value === item.unit_of_measure);
    setUnitChoice(knownUnit ? item.unit_of_measure : 'other');
    setCustomUnit(knownUnit ? '' : item.unit_of_measure);
    setCategoryID(String(item.category_id));
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
      const unitOfMeasure = unitChoice === 'other' ? customUnit.trim() : unitChoice;
      const payload = JSON.stringify({ name, sku, description, unit_of_measure: unitOfMeasure, category_id: Number(categoryID), steps: steps.map(({ title, instructions }) => ({ title, instructions })) });
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

  function closeItemForm() {
    if (!saving) {
      setShowItemForm(false);
      setError('');
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

  return (
    <main className="workspace-shell">
      <section className="workspace-content">
        <div className="workspace-heading">
          <div><span className="panel-kicker">Catalog</span><h1>Items and categories.</h1><p>Organize the items used across your company.</p></div>
          <div className="company-actions">
            {canViewItems && <button className={view === 'items' ? 'primary-button compact-button' : 'quiet-button'} onClick={() => setView('items')}>Items</button>}
            {canViewCategories && <button className={view === 'categories' ? 'primary-button compact-button' : 'quiet-button'} onClick={() => setView('categories')}>Categories</button>}
          </div>
        </div>
        {error && !showItemForm && <div className="form-error" role="alert">{error}</div>}

        {view === 'items' && canViewItems && <>
          <div className="workspace-heading catalog-section-heading"><div><span className="panel-kicker">Item catalog</span><h2>Company items</h2></div>{canManageItems && <button className="primary-button compact-button" disabled={loading} onClick={startCreate}>Add item</button>}</div>
          {loading ? <div className="directory-message">Loading items...</div> : items.length === 0 ? <div className="empty-state">No items yet. Add the first company item.</div> : <div className="warehouse-list">{items.map((item) => <article className="warehouse-row item-row" key={item.id}><div className="item-summary"><strong>{item.name}</strong><span>{item.sku || 'No SKU'} · {item.unit_of_measure} · {item.category_name}{item.description ? ` · ${item.description}` : ''}</span><ol>{item.steps.map((step) => <li key={step.id}>{step.title}</li>)}</ol></div><div className="item-row-actions"><span className={`status-badge ${item.is_active ? 'active' : ''}`}>{item.is_active ? 'Active' : 'Archived'}</span>{canManageItems && item.is_active && <div className="company-actions"><button className="quiet-button" onClick={() => startEdit(item)}>Edit</button><button className="danger-button" onClick={() => void archiveItem(item)}>Archive</button></div>}</div></article>)}</div>}
        </>}

        {view === 'categories' && canViewCategories && <CategoryManager categories={categories} canManage={canManageCategories} onChanged={loadCategories} />}
      </section>

      {showItemForm && <div className="modal-backdrop item-modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeItemForm(); }} onKeyDown={(event) => { if (event.key === 'Escape') closeItemForm(); }}>
        <section className="people-dialog item-dialog" role="dialog" aria-modal="true" aria-labelledby="item-dialog-title">
          <div className="dialog-heading">
            <div><span className="panel-kicker">{editing ? 'Update item' : 'New item'}</span><h2 id="item-dialog-title">{editing ? 'Edit item' : 'Add new item'}</h2><p>Set the item details, category, unit, and work steps.</p></div>
            <button className="dialog-close" type="button" aria-label="Close item form" disabled={saving} onClick={closeItemForm}>×</button>
          </div>
          <form className="item-form" onSubmit={saveItem} onInvalid={markFormSubmitted}>
            <div className="item-basic-fields">
              <label>Item name<input autoFocus value={name} onChange={(event) => setName(event.target.value)} maxLength={120} required /></label>
              <label>SKU<input value={sku} onChange={(event) => setSku(event.target.value)} maxLength={64} /></label>
              <label>Category<select value={categoryID} onChange={(event) => setCategoryID(event.target.value)} required disabled={loading || !canViewCategories || categories.length === 0}><option value="">Select a category</option>{categories.map((category) => <option key={category.id} value={category.id}>{category.name}</option>)}</select></label>
              <label>Unit of measure<select value={unitChoice} onChange={(event) => setUnitChoice(event.target.value)} required>{unitChoices.map((unit) => <option key={unit.value} value={unit.value}>{unit.label}</option>)}<option value="other">Other</option></select></label>
              {unitChoice === 'other' && <label>Custom unit<input value={customUnit} onChange={(event) => setCustomUnit(event.target.value)} maxLength={40} placeholder="e.g. pallet" required /></label>}
              <label className="item-description-field">Description<textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} rows={2} /></label>
            </div>
            {loading && <div className="operation-log-note" role="status">Loading categories...</div>}
            {!loading && !canViewCategories && <div className="form-error" role="alert">Category viewing permission is required to assign an item category.</div>}
            {!loading && canViewCategories && categories.length === 0 && <div className="form-error" role="alert">Create a category before adding or editing an item.</div>}
            <div className="item-steps-heading"><div><span className="panel-kicker">Work sequence</span><h3>Steps</h3></div><button className="quiet-button" type="button" onClick={addStep}>Add step</button></div>
            <div className="item-step-list">{steps.map((step, index) => <section className="item-step-card" key={step.key}>
              <div className="item-step-card-heading"><strong>Step {index + 1}</strong><div className="company-actions"><button className="quiet-button" type="button" disabled={index === 0} onClick={() => moveStep(index, -1)} aria-label={`Move step ${index + 1} up`}>Move up</button><button className="quiet-button" type="button" disabled={index === steps.length - 1} onClick={() => moveStep(index, 1)} aria-label={`Move step ${index + 1} down`}>Move down</button><button className="danger-button" type="button" disabled={steps.length === 1} onClick={() => setSteps((current) => current.filter((item) => item.key !== step.key))}>Remove</button></div></div>
              <label>Step name<input value={step.title} onChange={(event) => updateStep(step.key, 'title', event.target.value)} maxLength={120} required /></label>
              <label>Instructions<textarea value={step.instructions} onChange={(event) => updateStep(step.key, 'instructions', event.target.value)} maxLength={2000} rows={3} /></label>
            </section>)}</div>
            {error && <div className="form-error" role="alert">{error}</div>}
            <div className="dialog-footer"><button className="quiet-button" type="button" disabled={saving} onClick={closeItemForm}>Cancel</button><button className="primary-button" type="submit" disabled={saving || loading || !canViewCategories || categories.length === 0}>{saving ? 'Saving...' : editing ? 'Save changes' : 'Create item'}</button></div>
          </form>
        </section>
      </div>}
    </main>
  );
}