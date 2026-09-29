import { FormEvent, useEffect, useState } from 'react';

type User = {
  id: number;
  name: string;
  email: string;
  company_id: number;
  company_slug: string;
};

type Permissions = {
  company: string[];
  warehouses: Record<string, string[]>;
};

type SessionResponse = {
  access_token: string;
  expires_in: number;
  user: User;
  permissions: Permissions;
};

type MeResponse = {
  user: User;
  permissions: Permissions;
};

type ApiError = Error & { code?: string; status?: number };

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';

let accessToken = '';

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  });

  if (response.status === 401 && retry && path !== '/auth/refresh' && path !== '/auth/login') {
    try {
      const refreshed = await request<SessionResponse>('/auth/refresh', { method: 'POST' }, false);
      accessToken = refreshed.access_token;
      return request<T>(path, init, false);
    } catch {
      accessToken = '';
    }
  }

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: { code?: string; message?: string } } | null;
    const error = new Error(body?.error?.message ?? 'Request failed') as ApiError;
    error.code = body?.error?.code;
    error.status = response.status;
    throw error;
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

function LoginView({ onLogin }: { onLogin: (session: SessionResponse) => void }) {
  const [companySlug, setCompanySlug] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      const session = await request<SessionResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ company_slug: companySlug, email, password }),
      });
      accessToken = session.access_token;
      onLogin(session);
      setPassword('');
    } catch (loginError) {
      const typedError = loginError as ApiError;
      setError(typedError.code === 'invalid_credentials' ? 'The company, email, or password is not valid.' : typedError.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-layout">
      <section className="auth-intro">
        <span className="eyebrow">ProductionLineFlow</span>
        <h1>Keep every warehouse moving.</h1>
        <p>Sign in to coordinate people, places, and production work from one operational view.</p>
      </section>
      <section className="login-panel" aria-labelledby="login-title">
        <div className="panel-heading">
          <span className="panel-kicker">Workspace access</span>
          <h2 id="login-title">Sign in</h2>
          <p>Use your company workspace credentials.</p>
        </div>
        <form onSubmit={submit}>
          <label>
            Company workspace
            <input value={companySlug} onChange={(event) => setCompanySlug(event.target.value)} autoComplete="organization" required placeholder="acme" />
          </label>
          <label>
            Email
            <input type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username" required placeholder="you@company.com" />
          </label>
          <label>
            Password
            <input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required />
          </label>
          {error && <div className="form-error" role="alert">{error}</div>}
          <button className="primary-button" type="submit" disabled={submitting}>
            {submitting ? 'Signing in...' : 'Sign in'}
          </button>
        </form>
      </section>
    </main>
  );
}

function Workspace({ user, permissions, onLogout }: { user: User; permissions: Permissions; onLogout: () => void }) {
  const navigation = [
    { label: 'Warehouses', permission: 'warehouse.view' },
    { label: 'People & roles', permission: 'users.create' },
    { label: 'Operations', permission: 'workers.tasks.execute' },
  ].filter((item) => permissions.company.includes(item.permission));

  return (
    <main className="workspace-shell">
      <header className="topbar">
        <div>
          <span className="eyebrow">{user.company_slug}</span>
          <strong>ProductionLineFlow</strong>
        </div>
        <div className="user-menu">
          <span>{user.name}</span>
          <button className="quiet-button" onClick={onLogout}>Sign out</button>
        </div>
      </header>
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
            <button className="workspace-card" key={item.label}>
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

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [permissions, setPermissions] = useState<Permissions>({ company: [], warehouses: {} });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    request<SessionResponse>('/auth/refresh', { method: 'POST' })
      .then((session) => {
        accessToken = session.access_token;
        return request<MeResponse>('/me');
      })
      .then((me) => {
        if (active) {
          setUser(me.user);
          setPermissions(me.permissions);
        }
      })
      .catch(() => {
        accessToken = '';
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
    accessToken = '';
    setUser(null);
    setPermissions({ company: [], warehouses: {} });
  }

  if (loading) {
    return <main className="loading-screen"><span className="loading-dot" />Restoring your session...</main>;
  }
  if (!user) {
    return <LoginView onLogin={(session) => { setUser(session.user); setPermissions(session.permissions); }} />;
  }
  return <Workspace user={user} permissions={permissions} onLogout={logout} />;
}
