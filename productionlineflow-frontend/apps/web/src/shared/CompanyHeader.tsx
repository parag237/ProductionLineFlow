import { useEffect, useRef, useState, type FormEvent } from 'react';
import { request } from '../api/client';
import type { ApiError, User } from '../types';

export default function CompanyHeader({ user, onLogout, onBack }: { user: User; onLogout: () => Promise<void>; onBack?: () => void }) {
  const [open, setOpen] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [saving, setSaving] = useState(false);
  const profileRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function closeOnOutsideClick(event: PointerEvent) {
      if (event.target instanceof Node && !profileRef.current?.contains(event.target)) setOpen(false);
    }
    document.addEventListener('pointerdown', closeOnOutsideClick);
    return () => document.removeEventListener('pointerdown', closeOnOutsideClick);
  }, [open]);

  async function changePassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(''); setMessage('');
    if (newPassword !== confirmation) { setError('The passwords do not match.'); return; }
    setSaving(true);
    try {
      await request('/me/password', { method: 'PATCH', body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) });
      setCurrentPassword(''); setNewPassword(''); setConfirmation(''); setMessage('Password changed successfully.');
    } catch (e) { setError((e as ApiError).message); }
    finally { setSaving(false); }
  }

  return <>
    <header className="topbar company-header"><div className="brand-block"><span className="eyebrow">{user.company_slug}</span><strong>ProductionLineFlow</strong></div>
      <div className="header-controls">{onBack && <button className="quiet-button header-back" onClick={onBack}>← Overview</button>}
        <div className="profile-menu-wrap" ref={profileRef}><button className="profile-trigger" aria-haspopup="true" aria-expanded={open} onClick={() => setOpen((value) => !value)}><span className="profile-avatar">{user.name.trim().charAt(0).toUpperCase()}</span><span className="profile-name">{user.name}</span><span aria-hidden="true">⌄</span></button>
          {open && <div className="profile-menu" role="menu"><div className="profile-menu-identity"><strong>{user.name}</strong><span>{user.email}</span></div><button role="menuitem" onClick={() => { setOpen(false); setError(''); setMessage(''); setPasswordOpen(true); }}>Change password</button><button role="menuitem" className="profile-logout" onClick={() => void onLogout()}>Log out</button></div>}
        </div>
      </div>
    </header>
    {passwordOpen && <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget && !saving) setPasswordOpen(false); }}><section className="people-dialog" role="dialog" aria-modal="true" aria-labelledby="change-password-title"><div className="dialog-heading"><div><span className="panel-kicker">Account security</span><h2 id="change-password-title">Change password</h2><p>Update the password for {user.email}.</p></div><button className="dialog-close" aria-label="Close change password" disabled={saving} onClick={() => setPasswordOpen(false)}>×</button></div><form className="password-reset-form" onSubmit={changePassword}><label>Current password<input type="password" autoComplete="current-password" required value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} /></label><label>New password<input type="password" autoComplete="new-password" minLength={10} required value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></label><label>Confirm new password<input type="password" autoComplete="new-password" minLength={10} required value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label><small>Use at least 10 characters.</small>{error && <div className="form-error" role="alert">{error}</div>}{message && <div className="success-message" role="status">{message}</div>}<div className="dialog-footer"><button className="quiet-button" type="button" disabled={saving} onClick={() => setPasswordOpen(false)}>Close</button><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Change password'}</button></div></form></section></div>}
  </>;
}
