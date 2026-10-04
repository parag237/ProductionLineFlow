package server

import (
	"net/http"

	"productionlineflow-api/internal/config"
	"productionlineflow-api/internal/constants"

	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config, dependencySets ...*Dependencies) *gin.Engine {
	if cfg != nil && cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	var deps *Dependencies
	if len(dependencySets) > 0 {
		deps = dependencySets[0]
	}

	r := gin.New()
	r.Use(requestLogging(), gin.Recovery(), corsMiddleware(cfg))

	v1 := r.Group("/api/v1")
	registerHealthRoutes(r, v1)
	registerPlatformRoutes(r, v1, deps)
	authHandler := registerTenantAuthRoutes(r, v1, deps)
	registerTenantRoutes(v1, deps, authHandler)
	return r
}

func registerHealthRoutes(root *gin.Engine, versioned *gin.RouterGroup) {
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	root.GET("/", healthHandler)
	root.HEAD("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	root.GET("/health", healthHandler)
	versioned.GET("/health", healthHandler)
}

func registerPlatformRoutes(root gin.IRoutes, versioned *gin.RouterGroup, deps *Dependencies) {
	platform := versioned.Group("/platform")
	var platformHandler *PlatformHandler
	platformLogin := gin.HandlerFunc(authUnavailable)
	platformRefresh := gin.HandlerFunc(authUnavailable)
	platformLogout := gin.HandlerFunc(authUnavailable)
	if deps != nil && deps.PlatformAuth != nil {
		platformHandler = NewPlatformHandler(deps.PlatformAuth, deps.Company, deps.SecureCookie)
		platformLogin = platformHandler.Login
		platformRefresh = withPlatformSessionFromAccessToken(deps.JWT, platformHandler.Refresh)
		platformLogout = withPlatformSessionFromAccessToken(deps.JWT, platformHandler.Logout)
		platformSecured := platform.Group("")
		if deps.JWT != nil && deps.PlatformRepository != nil && deps.PlatformAuth != nil {
			platformSecured.Use(RequirePlatformAuth(deps.JWT, deps.PlatformRepository, deps.PlatformAuth.RefreshTTL(), deps.SecureCookie))
		}
		platformSecured.GET("/me", platformHandler.Me)
		platformSecured.POST("/companies", RequirePlatformPermission(constants.PermissionCompaniesCreate), platformHandler.CreateCompany)
		platformSecured.GET("/companies", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.ListCompanies)
		platformSecured.PATCH("/companies/:id", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.UpdateCompany)
		platformSecured.POST("/companies/:id/suspend", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.SuspendCompany)
		platformSecured.POST("/companies/:id/reactivate", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.ReactivateCompany)
	}
	registerAuthRoutes(root, versioned, "/platform", platformLogin, platformRefresh, platformLogout)
}

func registerTenantAuthRoutes(root gin.IRoutes, versioned *gin.RouterGroup, deps *Dependencies) *AuthHandler {
	var authHandler *AuthHandler
	tenantLogin := gin.HandlerFunc(authUnavailable)
	tenantRefresh := gin.HandlerFunc(authUnavailable)
	tenantLogout := gin.HandlerFunc(authUnavailable)
	if deps != nil && deps.Auth != nil {
		authHandler = NewAuthHandler(deps.Auth, deps.SecureCookie)
		tenantLogin = authHandler.Login
		tenantRefresh = withTenantSessionFromAccessToken(deps.JWT, authHandler.Refresh)
		tenantLogout = withTenantSessionFromAccessToken(deps.JWT, authHandler.Logout)
	}
	registerAuthRoutes(root, versioned, "", tenantLogin, tenantRefresh, tenantLogout)
	return authHandler
}

func registerTenantRoutes(versioned *gin.RouterGroup, deps *Dependencies, authHandler *AuthHandler) {
	secured := versioned.Group("")
	if deps != nil && deps.JWT != nil && deps.Repository != nil && deps.Auth != nil {
		secured.Use(RequireTenantAuth(deps.JWT, deps.Repository, deps.Auth.RefreshTTL(), deps.SecureCookie))
	}
	if authHandler != nil {
		secured.GET("/me", authHandler.Me)
	} else {
		secured.GET("/me", authUnavailable)
	}
	secured.GET("/navigation", func(c *gin.Context) {
		c.JSON(200, gin.H{"items": []string{"warehouses", "users", "roles"}})
	})
	secured.GET("/screens/:name", func(c *gin.Context) {
		c.JSON(200, gin.H{"screen": c.Param("name")})
	})
	secured.POST("/flows/start/:type", func(c *gin.Context) {
		c.JSON(200, gin.H{"flow_type": c.Param("type"), "state": "details"})
	})
	secured.GET("/flows/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"flow_id": c.Param("id"), "state": "details"})
	})
	secured.POST("/flows/:id/events", func(c *gin.Context) {
		c.JSON(200, gin.H{"flow_id": c.Param("id"), "state": "details"})
	})
	registerWarehouseRoutes(secured, deps)
	registerPeopleRoutes(secured, deps)
	registerItemRoutes(secured, deps)
	registerOperationLogRoutes(secured, deps)
}

func registerOperationLogRoutes(secured *gin.RouterGroup, deps *Dependencies) {
	if deps == nil || deps.OperationLogs == nil {
		return
	}
	handler := NewOperationLogHandler(deps.OperationLogs)
	secured.GET("/operation-logs/options", handler.Options)
	secured.GET("/operation-logs", handler.List)
	secured.POST("/operation-logs", handler.Create)
}

func registerWarehouseRoutes(secured *gin.RouterGroup, deps *Dependencies) {
	if deps != nil && deps.Warehouse != nil {
		warehouseHandler := NewWarehouseHandler(deps.Warehouse)
		secured.GET("/warehouses", warehouseHandler.List)
		secured.POST("/warehouses", warehouseHandler.Create)
		secured.GET("/warehouses/:id", warehouseHandler.Get)
		secured.PATCH("/warehouses/:id", warehouseHandler.Update)
		secured.DELETE("/warehouses/:id", warehouseHandler.Delete)
	}
}

func registerPeopleRoutes(secured *gin.RouterGroup, deps *Dependencies) {
	if deps != nil && deps.People != nil {
		peopleHandler := NewPeopleHandler(deps.People)
		secured.GET("/users", peopleHandler.List)
		secured.GET("/users/:id", peopleHandler.Get)
		secured.POST("/users", peopleHandler.Create)
		secured.PATCH("/users/:id", peopleHandler.Update)
		secured.DELETE("/users/:id", peopleHandler.Deactivate)
		secured.PATCH("/users/:id/password", peopleHandler.ResetPassword)
		secured.PATCH("/me/password", peopleHandler.ChangeOwnPassword)
		secured.POST("/users/:id/role-assignments", peopleHandler.AddAssignment)
		secured.DELETE("/users/:id/role-assignments/:assignmentId", peopleHandler.RemoveAssignment)
		secured.POST("/users/:id/transfer-super-admin", peopleHandler.TransferSuperAdmin)
		secured.GET("/roles", peopleHandler.ListRoles)
		secured.GET("/permissions", peopleHandler.ListPermissionCatalog)
		secured.POST("/roles", peopleHandler.CreateRole)
		secured.PATCH("/roles/:id", peopleHandler.UpdateRole)
		secured.DELETE("/roles/:id", peopleHandler.DeleteRole)
	}
}

func registerItemRoutes(secured *gin.RouterGroup, deps *Dependencies) {
	if deps == nil || (deps.Items == nil && deps.Production == nil) {
		return
	}
	handler := NewItemHandler(deps.Items, deps.Production)
	if deps.Items != nil {
		secured.GET("/items", handler.List)
		secured.POST("/items", handler.Create)
		secured.GET("/items/:id", handler.Get)
		secured.PATCH("/items/:id", handler.Update)
		secured.DELETE("/items/:id", handler.Archive)
	}
	if deps.Production != nil {
		secured.GET("/production/runs", handler.ListRuns)
		secured.POST("/production/runs", handler.CreateRun)
		secured.GET("/production/runs/:runID", handler.GetRun)
		secured.POST("/production/runs/:runID/steps/:stepID/complete", handler.CompleteStep)
	}
}

func registerAuthRoutes(root, versioned gin.IRoutes, prefix string, login, refresh, logout gin.HandlerFunc) {
	authPath := prefix + "/auth"
	root.POST(authPath+"/login", login)
	root.POST(authPath+"/refresh", refresh)
	root.POST(authPath+"/logout", logout)
	versioned.POST(authPath+"/login", login)
	versioned.POST(authPath+"/refresh", refresh)
	versioned.POST(authPath+"/logout", logout)
}

func authUnavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "auth_unavailable", "message": "Authentication service is unavailable"}})
}

func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if cfg != nil {
			for _, allowedOrigin := range cfg.Server.AllowedOrigins {
				if origin == allowedOrigin {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Refresh-Token")
					c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
					break
				}
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}
