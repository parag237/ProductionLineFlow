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
export type Category = { id: number; name: string; created_at: string; updated_at: string };
export type ItemStep = { id: number; position: number; title: string; instructions?: string };
export type Item = { id: number; name: string; sku?: string; description?: string; unit_of_measure: string; category_id: number; category_name: string; is_active: boolean; created_at: string; updated_at: string; steps: ItemStep[] };
export type OperationLogStepOption = { id: number; title: string };
export type OperationLogItemOption = { id: number; name: string; category_name: string; unit_of_measure: string; steps: OperationLogStepOption[] };
export type OperationLogWarehouseOption = { id: number; name: string };
export type OperationLogPerformerOption = { id: number; name: string };
export type OperationLogOptions = { warehouses: OperationLogWarehouseOption[]; items: OperationLogItemOption[]; performers: OperationLogPerformerOption[]; today: string };
export type OperationLogEntry = { id: number; warehouse_id: number; warehouse_name: string; work_date: string; item_id: number; item_name: string; category_name: string; step_id: number; step_title: string; unit_of_measure: string; quantity: number; performed_by: number; performer_name: string; recorded_by: number; created_at: string };
export type ItemStepInput = { title: string; instructions: string };
export type Person = { id: number; name: string; email: string; is_active: boolean; assignments: Assignment[] };
export type Assignment = { id: number; role_id: number; role_slug: string; role_name: string; role_scope: string; warehouse_id: number | null; warehouse_name?: string };
export type Role = { id: number; slug: string; name: string; scope: string; is_system: boolean; permissions: string[] };
export type Permission = { key: string; description: string; audience: 'company' | 'platform' };

export type ApiError = Error & { code?: string; status?: number };

export type PlatformUser = { id: number; name: string; email: string };
export type PlatformSession = { access_token: string; refresh_token: string; session_id: string; expires_in: number; idle_timeout_seconds: number; user: PlatformUser; permissions: string[] };
export type PlatformCompany = { id: number; slug: string; name: string; status: 'active' | 'suspended'; suspended_at?: string; activated_at?: string };
