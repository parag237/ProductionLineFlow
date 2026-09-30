export type User = {
  id: number;
  name: string;
  email: string;
  company_id: number;
  company_slug: string;
};

export type Permissions = {
  company: string[];
  warehouses: Record<string, string[]>;
};

export function normalizePermissions(value: Partial<Permissions> | null | undefined): Permissions {
  const warehouses = value?.warehouses;
  const normalizedWarehouses: Record<string, string[]> = {};
  if (warehouses && typeof warehouses === 'object' && !Array.isArray(warehouses)) {
    for (const [warehouseID, list] of Object.entries(warehouses)) {
      if (Array.isArray(list)) normalizedWarehouses[warehouseID] = list.filter((permission): permission is string => typeof permission === 'string');
    }
  }
  return {
    company: Array.isArray(value?.company) ? value.company.filter((permission): permission is string => typeof permission === 'string') : [],
    warehouses: normalizedWarehouses,
  };
}

export type SessionResponse = {
  access_token: string;
  refresh_token: string;
  session_id: string;
  expires_in: number;
  idle_timeout_seconds: number;
  user: User;
  permissions: Permissions;
};

export type MeResponse = {
  user: User;
  permissions: Permissions;
};
export type Warehouse = { id: number; name: string; type_name: string; address?: string; state: string; created_at: string };
export type Person = { id: number; name: string; email: string; is_active: boolean; assignments: Assignment[] };
export type Assignment = { id: number; role_id: number; role_slug: string; role_name: string; role_scope: string; warehouse_id: number | null; warehouse_name?: string };
export type Role = { id: number; slug: string; name: string; scope: string; is_system: boolean; permissions: string[] };
export type Permission = { key: string; description: string };

export type ApiError = Error & { code?: string; status?: number };

export type PlatformUser = { id: number; name: string; email: string };
export type PlatformSession = { access_token: string; refresh_token: string; session_id: string; expires_in: number; idle_timeout_seconds: number; user: PlatformUser; permissions: string[] };
export type PlatformCompany = { id: number; slug: string; name: string; status: 'active' | 'suspended'; suspended_at?: string; activated_at?: string };
