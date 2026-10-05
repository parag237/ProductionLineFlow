import { useEffect, useState } from 'react';
import { request } from '../../api/client';
import type { ApiError, OperationLogAnalysis, OperationLogOptions, Permissions } from '../../types';

function localDate(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function hasPermission(permissions: Permissions, permission: string, warehouseID: number) {
  return permissions.company.includes(permission) || permissions.warehouses[String(warehouseID)]?.includes(permission) === true;
}

function formatQuantity(quantity: number) {
  return quantity.toLocaleString(undefined, { maximumFractionDigits: 6 });
}

function formatDay(value: string) {
  return new Date(`${value}T12:00:00`).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export default function OperationsAnalysis({ permissions }: { permissions: Permissions }) {
  const canAnalyze = permissions.company.includes('operations.logs.view') || permissions.company.includes('operations.logs.edit') || Object.values(permissions.warehouses).some((list) => list.includes('operations.logs.view') || list.includes('operations.logs.edit'));
  const today = localDate(new Date());
  const thirtyDaysAgo = new Date();
  thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 29);

  const [warehouses, setWarehouses] = useState<OperationLogOptions['warehouses']>([]);
  const [warehouseID, setWarehouseID] = useState('all');
  const [fromDate, setFromDate] = useState(localDate(thirtyDaysAgo));
  const [toDate, setToDate] = useState(today);
  const [analysis, setAnalysis] = useState<OperationLogAnalysis | null>(null);
  const [loadingWarehouses, setLoadingWarehouses] = useState(true);
  const [loadingAnalysis, setLoadingAnalysis] = useState(false);
  const [error, setError] = useState('');

  const visibleWarehouses = warehouses.filter((warehouse) => hasPermission(permissions, 'operations.logs.view', warehouse.id) || hasPermission(permissions, 'operations.logs.edit', warehouse.id));
  const maxDailyEntries = Math.max(1, ...(analysis?.daily.map((total) => total.entry_count) ?? []));

  useEffect(() => {
    let active = true;
    request<OperationLogOptions>('/operation-logs/options')
      .then((options) => {
        if (active) setWarehouses(options.warehouses ?? []);
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
    if (!canAnalyze || !fromDate || !toDate || fromDate > toDate) {
      setAnalysis(null);
      setLoadingAnalysis(false);
      return;
    }
    let active = true;
    setLoadingAnalysis(true);
    setError('');
    const query = new URLSearchParams({ from_date: fromDate, to_date: toDate, warehouse_id: warehouseID || 'all' });
    request<OperationLogAnalysis>(`/operation-logs/analysis?${query.toString()}`)
      .then((result) => {
        if (active) setAnalysis(result);
      })
      .catch((loadError) => {
        if (active) setError((loadError as ApiError).message);
      })
      .finally(() => {
        if (active) setLoadingAnalysis(false);
      });
    return () => { active = false; };
  }, [canAnalyze, fromDate, toDate, warehouseID]);

  return <section className="operations-analysis">
    <div className="analysis-toolbar">
      <div><span className="panel-kicker">Work-log analytics</span><p>Output and activity recorded across items and steps.</p></div>
      <div className="analysis-filters">
        <label>Warehouse<select value={warehouseID} onChange={(event) => setWarehouseID(event.target.value)} disabled={loadingWarehouses || visibleWarehouses.length === 0}>
          <option value="all">All warehouses</option>
          {visibleWarehouses.map((warehouse) => <option key={warehouse.id} value={warehouse.id}>{warehouse.name}</option>)}
        </select></label>
        <label>From<input type="date" value={fromDate} max={toDate} onChange={(event) => setFromDate(event.target.value)} /></label>
        <label>To<input type="date" value={toDate} min={fromDate} max={today} onChange={(event) => setToDate(event.target.value)} /></label>
      </div>
    </div>

    {error && <div className="form-error" role="alert">{error}</div>}
    {!canAnalyze ? <div className="analysis-empty">Your role does not have permission to view work-log analysis.</div> : loadingWarehouses ? <div className="directory-message">Loading analysis filters...</div> : visibleWarehouses.length === 0 ? <div className="analysis-empty">No warehouses are available for your analysis permissions.</div> : loadingAnalysis ? <div className="directory-message">Updating analysis...</div> : !analysis || analysis.summary.entry_count === 0 ? <div className="analysis-empty">No work entries match this date range and warehouse.</div> : <>
      <section className="analysis-summary-grid" aria-label="Analysis summary">
        <article className="analysis-summary-item"><span>Work entries</span><strong>{analysis.summary.entry_count.toLocaleString()}</strong></article>
        <article className="analysis-summary-item"><span>Items worked</span><strong>{analysis.summary.item_count.toLocaleString()}</strong></article>
        <article className="analysis-summary-item"><span>Item steps</span><strong>{analysis.summary.step_count.toLocaleString()}</strong></article>
        <article className="analysis-summary-item"><span>Workers involved</span><strong>{analysis.summary.performer_count.toLocaleString()}</strong></article>
      </section>

      <section className="analysis-section">
        <div className="analysis-section-heading"><div><span className="panel-kicker">Recorded output</span><h2>Quantity by unit</h2></div></div>
        <div className="analysis-unit-totals">{analysis.units.map((total) => <div className="analysis-unit-total" key={total.unit_of_measure}><strong>{formatQuantity(total.quantity)}</strong><span>{total.unit_of_measure}</span></div>)}</div>
      </section>

      <section className="analysis-section">
        <div className="analysis-section-heading"><div><span className="panel-kicker">Activity over time</span><h2>Daily work entries</h2></div></div>
        <div className="analysis-daily-list">{analysis.daily.map((total) => <div className="analysis-daily-row" key={`${total.work_date}-${total.unit_of_measure}`}>
          <time dateTime={total.work_date}>{formatDay(total.work_date)}</time>
          <div className="analysis-daily-bar"><span style={{ width: `${Math.max(3, total.entry_count / maxDailyEntries * 100)}%` }} /></div>
          <span className="analysis-daily-value">{total.entry_count} entries · {formatQuantity(total.quantity)} {total.unit_of_measure}</span>
        </div>)}</div>
      </section>

      <div className="analysis-breakdown-grid">
        <section className="analysis-section">
          <div className="analysis-section-heading"><div><span className="panel-kicker">Classification</span><h2>By category</h2></div></div>
          <div className="analysis-table-wrap"><table className="analysis-table"><thead><tr><th>Category</th><th>Entries</th><th>Quantity</th></tr></thead><tbody>{analysis.categories.map((row) => <tr key={`${row.category_name}-${row.unit_of_measure}`}><td>{row.category_name}</td><td>{row.entry_count}</td><td>{formatQuantity(row.quantity)} {row.unit_of_measure}</td></tr>)}</tbody></table></div>
        </section>
        <section className="analysis-section">
          <div className="analysis-section-heading"><div><span className="panel-kicker">Catalog</span><h2>By item</h2></div></div>
          <div className="analysis-table-wrap"><table className="analysis-table"><thead><tr><th>Item</th><th>Category</th><th>Entries</th><th>Quantity</th></tr></thead><tbody>{analysis.items.map((row) => <tr key={`${row.item_id}-${row.unit_of_measure}`}><td>{row.item_name}</td><td>{row.category_name}</td><td>{row.entry_count}</td><td>{formatQuantity(row.quantity)} {row.unit_of_measure}</td></tr>)}</tbody></table></div>
        </section>
        <section className="analysis-section">
          <div className="analysis-section-heading"><div><span className="panel-kicker">Work sequence</span><h2>By step</h2></div></div>
          <div className="analysis-table-wrap"><table className="analysis-table"><thead><tr><th>Item / step</th><th>Entries</th><th>Quantity</th></tr></thead><tbody>{analysis.steps.map((row) => <tr key={`${row.item_name}-${row.step_title}-${row.unit_of_measure}`}><td><strong>{row.item_name}</strong><span>{row.step_title}</span></td><td>{row.entry_count}</td><td>{formatQuantity(row.quantity)} {row.unit_of_measure}</td></tr>)}</tbody></table></div>
        </section>
        <section className="analysis-section">
          <div className="analysis-section-heading"><div><span className="panel-kicker">People</span><h2>By worker</h2></div></div>
          <div className="analysis-table-wrap"><table className="analysis-table"><thead><tr><th>Worker</th><th>Entries</th><th>Quantity</th></tr></thead><tbody>{analysis.performers.map((row) => <tr key={`${row.performer_id}-${row.unit_of_measure}`}><td>{row.performer_name}</td><td>{row.entry_count}</td><td>{formatQuantity(row.quantity)} {row.unit_of_measure}</td></tr>)}</tbody></table></div>
        </section>
      </div>
    </>}
  </section>;
}