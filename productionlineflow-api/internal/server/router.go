package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"productionlineflow-api/internal/config"
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
	secured.GET("/warehouses/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "state": "draft"})
	})
	secured.PATCH("/warehouses/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "state": "draft"})
	})
	secured.POST("/warehouses/:id/events", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "state": "draft"})
	})
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
