package server

import (
	"github.com/gin-gonic/gin"
	"warehouse-api/internal/config"
)

func NewRouter(cfg *config.Config) *gin.Engine {
	if cfg != nil && cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	v1.POST("/auth/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "login not implemented yet"})
	})
	v1.POST("/auth/refresh", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "refresh not implemented yet"})
	})
	v1.POST("/auth/logout", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "logout not implemented yet"})
	})
	v1.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": "demo-user", "company_id": "demo-company"})
	})
	v1.GET("/navigation", func(c *gin.Context) {
		c.JSON(200, gin.H{"items": []string{"warehouses", "users", "roles"}})
	})
	v1.GET("/screens/:name", func(c *gin.Context) {
		c.JSON(200, gin.H{"screen": c.Param("name")})
	})
	v1.POST("/flows/:type", func(c *gin.Context) {
		c.JSON(200, gin.H{"flow_type": c.Param("type"), "state": "details"})
	})
	v1.GET("/flows/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"flow_id": c.Param("id"), "state": "details"})
	})
	v1.POST("/flows/:id/events", func(c *gin.Context) {
		c.JSON(200, gin.H{"flow_id": c.Param("id"), "state": "details"})
	})
	v1.GET("/warehouses/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "state": "draft"})
	})
	v1.PATCH("/warehouses/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "state": "draft"})
	})
	v1.POST("/warehouses/:id/events", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "state": "draft"})
	})
	return r
}
