package constants

const (
	PasswordMemory      = 64 * 1024
	PasswordIterations  = 3
	PasswordParallelism = 2
	PasswordSaltLength  = 16
	PasswordKeyLength   = 32

	PlatformRoleProductOwner            = "product_owner"
	PermissionCompaniesCreate           = "companies.create"
	PermissionCompaniesManage           = "companies.manage"
	PermissionAdminsManage              = "admins.manage"
	PermissionRolesManage               = "roles.manage"
	PermissionUsersCreate               = "users.create"
	PermissionWarehouseMembersManage    = "warehouse.members.manage"
	PermissionWarehouseView             = "warehouse.view"
	PermissionWarehousesManage          = "warehouses.manage"
	PermissionItemsView                 = "items.view"
	PermissionItemsManage               = "items.manage"
	PermissionItemCategoriesView        = "items.categories.view"
	PermissionItemCategoriesManage      = "items.categories.manage"
	PermissionOperationLogsView         = "operations.logs.view"
	PermissionOperationLogsCreate       = "operations.logs.create"
	PermissionOperationLogsDateOverride = "operations.logs.date_override"

	UserContextKey                = "auth.user"
	PermissionsContextKey         = "auth.permissions"
	ActorContextKey               = "auth.actor"
	RefreshCookieName             = "productionlineflow_refresh"
	TenantAccessTokenType         = "tenant_access"
	PlatformAccessTokenType       = "platform_access"
	PlatformRefreshCookieName     = "productionlineflow_platform_refresh"
	PlatformUserContextKey        = "platform.user"
	PlatformPermissionsContextKey = "platform.permissions"
	PlatformActorContextKey       = "platform.actor"
)
