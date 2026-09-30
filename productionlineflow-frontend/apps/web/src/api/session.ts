import type { PlatformSession, SessionResponse } from '../types';

let accessToken = '';
let platformAccessToken = '';
let tenantRefreshToken = '';
let platformRefreshToken = '';
let tenantSessionID = '';
let platformSessionID = '';
let tenantIdleTimeout = 0;
let platformIdleTimeout = 0;
let tenantIdleTimer: number | undefined;
let platformIdleTimer: number | undefined;

export function setTenantAccessToken(token: string) { accessToken = token; }
export function setPlatformAccessToken(token: string) { platformAccessToken = token; }
export function getTenantAccessToken() { return accessToken; }
export function getPlatformAccessToken() { return platformAccessToken; }

const tenantRefreshStorageKey = 'productionlineflow:tenant:refresh-token';
const platformRefreshStorageKey = 'productionlineflow:platform:refresh-token';

function loadRefreshToken(platform: boolean) {
  try { return sessionStorage.getItem(platform ? platformRefreshStorageKey : tenantRefreshStorageKey) ?? ''; }
  catch { return ''; }
}

export function saveRefreshToken(platform: boolean, token: string) {
  if (platform) platformRefreshToken = token;
  else tenantRefreshToken = token;
  try {
    const key = platform ? platformRefreshStorageKey : tenantRefreshStorageKey;
    if (token) sessionStorage.setItem(key, token);
    else sessionStorage.removeItem(key);
  } catch { /* session storage may be disabled; the current tab can still use memory */ }
}

export function getRefreshToken(platform: boolean) {
  return platform
    ? platformRefreshToken || loadRefreshToken(true)
    : tenantRefreshToken || loadRefreshToken(false);
}

export function clearRefreshToken(platform: boolean) {
  saveRefreshToken(platform, '');
}

function idleStorageKey(platform: boolean, sessionID: string, suffix: string) {
  return `productionlineflow:${platform ? 'platform' : 'tenant'}:${sessionID}:${suffix}`;
}

function armIdleTimer(platform: boolean, activityAt = Date.now()) {
  const sessionID = platform ? platformSessionID : tenantSessionID;
  const timeout = platform ? platformIdleTimeout : tenantIdleTimeout;
  if (!sessionID || timeout <= 0) return;
  const timer = platform ? platformIdleTimer : tenantIdleTimer;
  if (timer !== undefined) window.clearTimeout(timer);
  const nextTimer = window.setTimeout(() => redirectToLogin(platform), Math.max(0, timeout * 1000 - (Date.now() - activityAt)));
  if (platform) platformIdleTimer = nextTimer;
  else tenantIdleTimer = nextTimer;
}

export function recordSessionActivity(platform: boolean) {
  const sessionID = platform ? platformSessionID : tenantSessionID;
  const timeout = platform ? platformIdleTimeout : tenantIdleTimeout;
  if (!sessionID || timeout <= 0) return;
  const activityAt = Date.now();
  armIdleTimer(platform, activityAt);
  try { localStorage.setItem(idleStorageKey(platform, sessionID, 'activity'), JSON.stringify({ activityAt })); } catch { /* browser storage may be disabled */ }
}

export function startIdleSession(platform: boolean, session: { session_id: string; idle_timeout_seconds: number }) {
  const previousID = platform ? platformSessionID : tenantSessionID;
  if (previousID && previousID !== session.session_id) {
    const previousTimer = platform ? platformIdleTimer : tenantIdleTimer;
    if (previousTimer !== undefined) window.clearTimeout(previousTimer);
  }
  if (platform) { platformSessionID = session.session_id; platformIdleTimeout = session.idle_timeout_seconds; }
  else { tenantSessionID = session.session_id; tenantIdleTimeout = session.idle_timeout_seconds; }
  recordSessionActivity(platform);
}

export function endIdleSession(platform: boolean) {
  const sessionID = platform ? platformSessionID : tenantSessionID;
  const timer = platform ? platformIdleTimer : tenantIdleTimer;
  if (timer !== undefined) window.clearTimeout(timer);
  try { if (sessionID) localStorage.setItem(idleStorageKey(platform, sessionID, 'ended'), String(Date.now())); } catch { /* browser storage may be disabled */ }
  if (platform) { platformIdleTimer = undefined; platformAccessToken = ''; platformSessionID = ''; platformIdleTimeout = 0; saveRefreshToken(true, ''); }
  else { tenantIdleTimer = undefined; accessToken = ''; tenantSessionID = ''; tenantIdleTimeout = 0; saveRefreshToken(false, ''); }
}

window.addEventListener('storage', (event) => {
  if (!event.key || !event.newValue) return;
  for (const platform of [false, true]) {
    const sessionID = platform ? platformSessionID : tenantSessionID;
    if (!sessionID || !event.key.startsWith(idleStorageKey(platform, sessionID, ''))) continue;
    if (event.key === idleStorageKey(platform, sessionID, 'ended')) { redirectToLogin(platform, false); break; }
    if (event.key === idleStorageKey(platform, sessionID, 'activity')) {
      try { const value = JSON.parse(event.newValue) as { activityAt?: number }; if (value.activityAt) armIdleTimer(platform, value.activityAt); } catch { /* ignore malformed cross-tab state */ }
      break;
    }
  }
});

export function redirectToLogin(platform = false, broadcast = true) {
  if (broadcast) endIdleSession(platform);
  else {
    const timer = platform ? platformIdleTimer : tenantIdleTimer;
    if (timer !== undefined) window.clearTimeout(timer);
	if (platform) { platformIdleTimer = undefined; platformAccessToken = ''; platformSessionID = ''; platformIdleTimeout = 0; saveRefreshToken(true, ''); }
	else { tenantIdleTimer = undefined; accessToken = ''; tenantSessionID = ''; tenantIdleTimeout = 0; saveRefreshToken(false, ''); }
  }
  const loginPath = platform ? '/platform/login' : '/login';
  if (window.location.pathname !== loginPath) window.location.replace(loginPath);
}

export function isLoginRoute(platform = false) {
  return window.location.pathname === (platform ? '/platform/login' : '/login');
}
