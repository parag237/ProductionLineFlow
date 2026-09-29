package constants

const (
	PasswordMemory      = 64 * 1024
	PasswordIterations  = 3
	PasswordParallelism = 2
	PasswordSaltLength  = 16
	PasswordKeyLength   = 32

	PlatformRoleProductOwner  = "product_owner"
	PermissionCompaniesCreate = "companies.create"

	UserContextKey        = "auth.user"
	PermissionsContextKey = "auth.permissions"
	ActorContextKey       = "auth.actor"
	RefreshCookieName     = "productionlineflow_refresh"
	TenantAccessTokenType = "tenant_access"
)
