import { useState, type FormEvent } from 'react';
import { platformRequest } from '../../api/client';
import { setPlatformAccessToken } from '../../api/session';
import { markFormSubmitted } from '../../shared/forms';
import type { ApiError, PlatformSession } from '../../types';

export default function PlatformLogin({ onLogin }: { onLogin: (session: PlatformSession) => void }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      const session = await platformRequest<PlatformSession>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      });
      setPlatformAccessToken(session.access_token);
      setPassword('');
      onLogin(session);
    } catch (loginError) {
      const typedError = loginError as ApiError;
      setError(typedError.code === 'invalid_platform_credentials' ? 'The platform credentials are not valid.' : typedError.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-layout">
      <section className="auth-intro">
        <span className="eyebrow">Platform operations</span>
        <h1>Build the next workspace.</h1>
        <p>Product Owner access for creating and provisioning ProductionLineFlow companies.</p>
      </section>
      <section className="login-panel" aria-labelledby="platform-login-title">
        <div className="panel-heading">
          <span className="panel-kicker">Operator access</span>
          <h2 id="platform-login-title">Platform sign in</h2>
          <p>This area is separate from tenant warehouse workspaces.</p>
        </div>
        <form onSubmit={submit} onInvalid={markFormSubmitted}>
          <label>Email<input type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="username" required /></label>
          <label>Password<input type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required /></label>
          {error && <div className="form-error" role="alert">{error}</div>}
          <button className="primary-button" type="submit" disabled={submitting}>{submitting ? 'Signing in...' : 'Enter platform'}</button>
        </form>
      </section>
    </main>
  );
}
