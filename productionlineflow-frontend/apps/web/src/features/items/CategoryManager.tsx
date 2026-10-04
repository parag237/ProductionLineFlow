import { useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import type { ApiError, Category } from '../../types';

export default function CategoryManager({ categories, canManage, onChanged }: { categories: Category[]; canManage: boolean; onChanged: () => Promise<void> }) {
  const [editing, setEditing] = useState<Category | null>(null);
  const [name, setName] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);

  function startCreate() {
    setEditing(null);
    setName('');
    setError('');
    setShowForm(true);
  }

  function startEdit(category: Category) {
    setEditing(category);
    setName(category.name);
    setError('');
    setShowForm(true);
  }

  async function saveCategory(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      if (editing) await request(`/categories/${editing.id}`, { method: 'PATCH', body: JSON.stringify({ name }) });
      else await request('/categories', { method: 'POST', body: JSON.stringify({ name }) });
      await onChanged();
      setEditing(null);
      setName('');
      setShowForm(false);
    } catch (saveError) {
      setError((saveError as ApiError).message);
    } finally {
      setSaving(false);
    }
  }

  async function deleteCategory(category: Category) {
    if (!window.confirm(`Delete ${category.name}? Categories assigned to items cannot be deleted.`)) return;
    setError('');
    try {
      await request(`/categories/${category.id}`, { method: 'DELETE' });
      await onChanged();
    } catch (deleteError) {
      setError((deleteError as ApiError).message);
    }
  }

  return <section className="category-manager">
    <div className="workspace-heading catalog-section-heading"><div><span className="panel-kicker">Item setup</span><h2>Categories</h2><p>Assign each item to one category.</p></div>{canManage && <button className="primary-button compact-button" onClick={startCreate}>Add category</button>}</div>
    {error && !editing && <div className="form-error" role="alert">{error}</div>}
    {categories.length === 0 ? <div className="empty-state">No categories are available.</div> : <div className="category-list">{categories.map((category) => <article className="category-row" key={category.id}>
      <div><strong>{category.name}</strong><span>Category</span></div>
      {canManage && <div className="company-actions"><button className="quiet-button" onClick={() => startEdit(category)}>Edit</button><button className="danger-button" onClick={() => void deleteCategory(category)}>Delete</button></div>}
    </article>)}</div>}

    {canManage && showForm && <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget && !saving) { setEditing(null); setName(''); setError(''); setShowForm(false); } }}>
      <section className="people-dialog category-dialog" role="dialog" aria-modal="true" aria-labelledby="category-dialog-title">
        <div className="dialog-heading"><div><span className="panel-kicker">Item setup</span><h2 id="category-dialog-title">{editing ? 'Edit category' : 'Add category'}</h2><p>Category names are unique within this company.</p></div><button className="dialog-close" type="button" aria-label="Close category form" disabled={saving} onClick={() => { setEditing(null); setName(''); setError(''); setShowForm(false); }}>×</button></div>
        <form className="category-form" onSubmit={saveCategory}><label>Category name<input autoFocus value={name} onChange={(event) => setName(event.target.value)} maxLength={80} required /></label>{error && <div className="form-error" role="alert">{error}</div>}<div className="dialog-footer"><button className="quiet-button" type="button" disabled={saving} onClick={() => { setEditing(null); setName(''); setError(''); setShowForm(false); }}>Cancel</button><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Saving...' : editing ? 'Save changes' : 'Create category'}</button></div></form>
      </section>
    </div>}
  </section>;
}