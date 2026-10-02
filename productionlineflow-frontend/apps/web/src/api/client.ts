import type { ApiError, PlatformSession, SessionResponse } from '../types';
import { getPlatformAccessToken, getRefreshToken, getTenantAccessToken, recordSessionActivity, redirectToLogin, saveRefreshToken, setPlatformAccessToken, setTenantAccessToken, startIdleSession } from './session';

const API_BASE = import.meta.env.VITE_API_BASE_URL;

let tenantRefreshPromise: Promise<SessionResponse> | null = null;
let platformRefreshPromise: Promise<PlatformSession> | null = null;

export function refreshTenantSession() {
  if (!tenantRefreshPromise) {
    const refresh = () => request<SessionResponse>('/auth/refresh', { method: 'POST' }, false);
    const locks = (navigator as Navigator & { locks?: { request<T>(name: string, callback: () => T | Promise<T>): Promise<Awaited<T>> } }).locks;
    tenantRefreshPromise = (locks ? locks.request<SessionResponse>('productionlineflow-tenant-refresh', refresh) : refresh()).finally(() => { tenantRefreshPromise = null; });
  }
  return tenantRefreshPromise!;
}

export function refreshPlatformSession() {
  if (!platformRefreshPromise) {
    const refresh = () => platformRequest<PlatformSession>('/auth/refresh', { method: 'POST' }, false);
    const locks = (navigator as Navigator & { locks?: { request<T>(name: string, callback: () => T | Promise<T>): Promise<Awaited<T>> } }).locks;
    platformRefreshPromise = (locks ? locks.request<PlatformSession>('productionlineflow-platform-refresh', refresh) : refresh()).finally(() => { platformRefreshPromise = null; });
  }
  return platformRefreshPromise!;
}

export async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  const currentAccessToken = getTenantAccessToken();
  if (currentAccessToken) headers.set('Authorization', `Bearer ${currentAccessToken}`);
	if (path === '/auth/refresh' || path === '/auth/logout') {
		const refreshToken = getRefreshToken(false);
		if (refreshToken) headers.set('X-Refresh-Token', refreshToken);
	}
	if (currentAccessToken && !path.startsWith('/auth/')) recordSessionActivity(false);

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'omit',
  });

  if (response.status === 401 && retry && path !== '/auth/refresh' && path !== '/auth/login') {
    try {
      const refreshed = await refreshTenantSession();
      setTenantAccessToken(refreshed.access_token);
      return request<T>(path, init, false);
    } catch {
      setTenantAccessToken('');
      saveRefreshToken(false, '');
      redirectToLogin(false);
    }
  }
  if (response.status === 401 && !retry && path !== '/auth/refresh' && path !== '/auth/login') redirectToLogin(false);

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: { code?: string; message?: string } } | null;
    const error = new Error(body?.error?.message ?? 'Request failed') as ApiError;
    error.code = body?.error?.code;
    error.status = response.status;
    throw error;
  }

  if (response.status === 204) {
    if (path === '/auth/login' || path === '/auth/refresh') return undefined as T;
    return undefined as T;
  }
  const result = await response.json() as T;
  if (path === '/auth/login' || path === '/auth/refresh') {
    const session = result as SessionResponse;
    saveRefreshToken(false, session.refresh_token);
    startIdleSession(false, session);
  }
  return result;
}



export async function platformRequest<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  const currentPlatformAccessToken = getPlatformAccessToken();
  if (currentPlatformAccessToken) headers.set('Authorization', `Bearer ${currentPlatformAccessToken}`);
	if (path === '/auth/refresh' || path === '/auth/logout') {
		const refreshToken = getRefreshToken(true);
		if (refreshToken) headers.set('X-Refresh-Token', refreshToken);
	}
	if (currentPlatformAccessToken && !path.startsWith('/auth/')) recordSessionActivity(true);
  const response = await fetch(`${API_BASE}/platform${path}`, { ...init, headers, credentials: 'omit' });
  if (response.status === 401 && retry && path !== '/auth/refresh' && path !== '/auth/login') {
    try {
      const refreshed = await refreshPlatformSession();
      setPlatformAccessToken(refreshed.access_token);
      return platformRequest<T>(path, init, false);
    } catch {
      setPlatformAccessToken('');
      saveRefreshToken(true, '');
      redirectToLogin(true);
    }
  }
  if (response.status === 401 && !retry && path !== '/auth/refresh' && path !== '/auth/login') redirectToLogin(true);
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: { code?: string; message?: string } } | null;
    const error = new Error(body?.error?.message ?? 'Request failed') as ApiError;
    error.code = body?.error?.code;
    error.status = response.status;
    throw error;
  }
  if (response.status === 204) return undefined as T;
  const result = await response.json() as T;
  if (path === '/auth/login' || path === '/auth/refresh') {
    const session = result as PlatformSession;
    saveRefreshToken(true, session.refresh_token);
    startIdleSession(true, session);
  }
  return result;
}
