import { useEffect, useState } from 'react';
import { refreshTenantSession, request } from './api/client';
import { clearRefreshToken, endIdleSession, isLoginRoute, redirectToLogin, setTenantAccessToken } from './api/session';
import LoginView from './features/auth/LoginView';
import CatalogDashboard from './features/items/CatalogDashboard';
import OperationsDashboard from './features/operations/OperationsDashboard';
import OperationsHub from './features/operations/OperationsHub';
import PeopleDashboard from './features/people/PeopleDashboard';
import PlatformApp from './features/platform/PlatformApp';
import WarehouseDashboard from './features/warehouses/WarehouseDashboard';
import Workspace from './features/workspace/Workspace';
import CompanyHeader from './shared/CompanyHeader';
import { normalizePermissions } from './types';
import type { MeResponse, Permissions, User } from './types';

export default function App() {
  if (window.location.pathname.startsWith('/platform')) return <PlatformApp />;
  const [user, setUser] = useState<User | null>(null);
  const [permissions, setPermissions] = useState<Permissions>({ company: [], warehouses: {} });
  const [loading, setLoading] = useState(true);
  const [showWarehouses, setShowWarehouses] = useState(false);
  const [showPeople, setShowPeople] = useState(false);
  const [showItems, setShowItems] = useState(false);
  const [showOperations, setShowOperations] = useState(false);
  const [operationsPage, setOperationsPage] = useState<'menu' | 'analysis' | 'workLog'>('menu');

  useEffect(() => {
    let active = true;
    if (isLoginRoute(false)) {
      clearRefreshToken(false);
      setLoading(false);
      return () => {
        active = false;
      };
    }
    refreshTenantSession()
      .then((session) => {
        setTenantAccessToken(session.access_token);
        return request<MeResponse>('/me');
      })
      .then((me) => {
        if (active) {
          setUser(me.user);
          setPermissions(normalizePermissions(me.permissions));
        }
      })
      .catch(() => {
        setTenantAccessToken('');
        if (active) redirectToLogin(false);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  async function logout() {
    await request('/auth/logout', { method: 'POST' }).catch(() => undefined);
    endIdleSession(false);
    setUser(null);
    setPermissions({ company: [], warehouses: {} });
  }

  function goBack() {
    if (showOperations && operationsPage !== 'menu') {
      setOperationsPage('menu');
      return;
    }
    setShowWarehouses(false);
    setShowPeople(false);
    setShowItems(false);
    setShowOperations(false);
    setOperationsPage('menu');
  }

  if (loading) {
    return <main className="loading-screen"><span className="loading-dot" />Restoring your session...</main>;
  }
  if (!user) {
    return <LoginView onLogin={(session) => { setUser(session.user); setPermissions(normalizePermissions(session.permissions)); }} />;
  }
  return <>
    <CompanyHeader user={user} onLogout={logout} onBack={showWarehouses || showPeople || showItems || showOperations ? goBack : undefined} />
    {showWarehouses ? <WarehouseDashboard canManage={permissions.company.includes('warehouses.manage')} /> : showPeople ? <PeopleDashboard user={user} permissions={permissions} onTransferred={logout} /> : showItems ? <CatalogDashboard permissions={permissions} /> : showOperations ? operationsPage === 'workLog' ? <OperationsDashboard permissions={permissions} /> : <OperationsHub page={operationsPage} permissions={permissions} onOpenAnalysis={() => setOperationsPage('analysis')} onOpenWorkLog={() => setOperationsPage('workLog')} /> : <Workspace user={user} permissions={permissions} onOpenWarehouses={() => setShowWarehouses(true)} onOpenPeople={() => setShowPeople(true)} onOpenItems={() => setShowItems(true)} onOpenOperations={() => { setOperationsPage('menu'); setShowOperations(true); }} />}
  </>;
}
