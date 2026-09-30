import { useEffect, useState } from 'react';
import { platformRequest, refreshPlatformSession } from '../../api/client';
import { clearRefreshToken, endIdleSession, isLoginRoute, redirectToLogin, setPlatformAccessToken } from '../../api/session';
import PlatformLogin from './PlatformLogin';
import PlatformWorkspace from './PlatformWorkspace';
import type { PlatformUser } from '../../types';

export default function PlatformApp() {
  const [user, setUser] = useState<PlatformUser | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    // Do not try to restore an old session while already on the login screen.
    // Besides being unnecessary, a rejected stale token can otherwise redirect
    // back to this exact URL repeatedly in some browsers.
    if (isLoginRoute(true)) {
      clearRefreshToken(true);
      setLoading(false);
      return () => { active = false; };
    }
    refreshPlatformSession()
      .then((session) => { setPlatformAccessToken(session.access_token); if (active) { setUser(session.user); setPermissions(session.permissions); } })
      .catch(() => { setPlatformAccessToken(''); if (active) redirectToLogin(true); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  async function logout() {
    await platformRequest('/auth/logout', { method: 'POST' }).catch(() => undefined);
    endIdleSession(true);
    setUser(null);
    setPermissions([]);
  }

  if (loading) return <main className="loading-screen"><span className="loading-dot" />Restoring platform session...</main>;
  if (!user) return <PlatformLogin onLogin={(session) => { setUser(session.user); setPermissions(session.permissions); }} />;
  return <PlatformWorkspace user={user} permissions={permissions} onLogout={logout} />;
}
