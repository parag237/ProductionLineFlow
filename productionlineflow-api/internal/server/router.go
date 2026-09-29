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
	r.Use(gin.Recovery(), corsMiddleware(cfg))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	platform := v1.Group("/platform")
	if deps == nil || deps.PlatformAuth == nil {
		platform.POST("/auth/login", authUnavailable)
		platform.POST("/auth/refresh", authUnavailable)
		platform.POST("/auth/logout", authUnavailable)
	} else {
		platformHandler := NewPlatformHandler(deps.PlatformAuth, deps.Company, deps.SecureCookie)
		platform.POST("/auth/login", platformHandler.Login)
		platform.POST("/auth/refresh", platformHandler.Refresh)
		platform.POST("/auth/logout", platformHandler.Logout)
		platformSecured := platform.Group("")
		if deps.JWT != nil && deps.PlatformRepository != nil {
			platformSecured.Use(RequirePlatformAuth(deps.JWT, deps.PlatformRepository))
		}
		platformSecured.GET("/me", platformHandler.Me)
		platformSecured.POST("/companies", RequirePlatformPermission(constants.PermissionCompaniesCreate), platformHandler.CreateCompany)
		platformSecured.GET("/companies", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.ListCompanies)
		platformSecured.PATCH("/companies/:id", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.UpdateCompany)
		platformSecured.POST("/companies/:id/suspend", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.SuspendCompany)
		platformSecured.POST("/companies/:id/reactivate", RequirePlatformPermission(constants.PermissionCompaniesManage), platformHandler.ReactivateCompany)
	}
	if deps == nil || deps.Auth == nil {
		v1.POST("/auth/login", authUnavailable)
		v1.POST("/auth/refresh", authUnavailable)
		v1.POST("/auth/logout", authUnavailable)
	} else {
		authHandler := NewAuthHandler(deps.Auth, deps.SecureCookie)
		v1.POST("/auth/login", authHandler.Login)
		v1.POST("/auth/refresh", authHandler.Refresh)
		v1.POST("/auth/logout", authHandler.Logout)
	}

	secured := v1.Group("")
	if deps != nil && deps.JWT != nil && deps.Repository != nil {
		secured.Use(RequireTenantAuth(deps.JWT, deps.Repository))
	}
	if deps != nil && deps.Auth != nil {
		authHandler := NewAuthHandler(deps.Auth, deps.SecureCookie)
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
	if deps != nil && deps.Warehouse != nil {
		warehouseHandler := NewWarehouseHandler(deps.Warehouse)
		secured.GET("/warehouses", warehouseHandler.List)
		secured.POST("/warehouses", warehouseHandler.Create)
		secured.GET("/warehouses/:id", warehouseHandler.Get)
		secured.PATCH("/warehouses/:id", warehouseHandler.Update)
		secured.DELETE("/warehouses/:id", warehouseHandler.Delete)
	}
	return r
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
					c.Header("Access-Control-Allow-Credentials", "true")
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
