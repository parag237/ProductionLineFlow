import type { Permissions, User } from '../../types';

export default function Workspace({ user, permissions, onOpenWarehouses, onOpenPeople, onOpenItems, onOpenOperations }: { user: User; permissions: Permissions; onOpenWarehouses: () => void; onOpenPeople: () => void; onOpenItems: () => void; onOpenOperations: () => void }) {
  const navigation = [
    { label: 'Warehouses', permission: 'warehouse.view' },
    { label: 'People & roles', permission: 'users.create' },
    { label: 'Items', permission: 'items.view' },
    { label: 'Operations', permission: 'operations.logs.view' },
  ].filter((item) => {
    if (item.label === 'People & roles') return permissions.company.includes('users.create') || Object.values(permissions.warehouses).some((list) => list.includes('warehouse.members.manage'));
    if (item.label === 'Items') return permissions.company.includes('items.view') || permissions.company.includes('items.manage') || permissions.company.includes('items.categories.view') || permissions.company.includes('items.categories.manage');
    if (item.label === 'Operations') return permissions.company.includes('operations.logs.view') || permissions.company.includes('operations.logs.create') || Object.values(permissions.warehouses).some((list) => list.includes('operations.logs.view') || list.includes('operations.logs.create'));
    if (item.permission === 'warehouse.view') return permissions.company.includes(item.permission) || Object.values(permissions.warehouses).some((list) => list.includes(item.permission));
    return permissions.company.includes(item.permission);
  });

  return (
    <main className="workspace-shell">
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
            <button className="workspace-card" key={item.label} onClick={item.label === 'Warehouses' ? onOpenWarehouses : item.label === 'People & roles' ? onOpenPeople : item.label === 'Items' ? onOpenItems : item.label === 'Operations' ? onOpenOperations : undefined}>
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
