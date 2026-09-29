package server

import (
	"net/http"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/constants"

	"github.com/gin-gonic/gin"
)

func RequireTenantAuth(jwtService *auth.JWTService, repository auth.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
			c.Abort()
			return
		}

		claims, err := jwtService.ValidateAccess(token)
		if err != nil {
			writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
			c.Abort()
			return
		}
		user, err := repository.FindUserByID(c.Request.Context(), claims.UserID, claims.CompanyID)
		if err != nil || !user.IsActive {
			writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
			c.Abort()
			return
		}
		if user.PermVersion != claims.PermVersion {
			writeError(c, http.StatusUnauthorized, "permissions_changed", "Permissions changed; refresh your session")
			c.Abort()
			return
		}
		permissions, err := repository.ListPermissions(c.Request.Context(), user.ID, user.CompanyID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", "Unable to load permissions")
			c.Abort()
			return
		}

		c.Set(constants.UserContextKey, user)
		c.Set(constants.PermissionsContextKey, permissions)
		c.Set(constants.ActorContextKey, actorFromPermissions(user, permissions))
		c.Next()
	}
}

func CurrentActor(c *gin.Context) (interface{}, bool) {
	value, exists := c.Get(constants.ActorContextKey)
	return value, exists
}
