import { useState, type FormEvent } from 'react';
import { request } from '../../api/client';
import { setTenantAccessToken } from '../../api/session';
import { markFormSubmitted } from '../../shared/forms';
import type { ApiError, SessionResponse } from '../../types';

export default function LoginView({ onLogin }: { onLogin: (session: SessionResponse) => void }) {
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
      setTenantAccessToken(session.access_token);
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
        <form onSubmit={submit} onInvalid={markFormSubmitted}>
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
