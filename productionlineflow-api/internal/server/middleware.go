package server

import (
	"net/http"
	"time"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/constants"

	"github.com/gin-gonic/gin"
)

func RequireTenantAuth(jwtService *auth.JWTService, repository auth.Repository, idleTimeout time.Duration, secureCookie bool) gin.HandlerFunc {
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
		setRequestSessionID(c, claims.SessionID)
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
		refreshCookie, _ := c.Cookie(constants.RefreshCookieName)
		active, cookieMatches, err := repository.TouchTenantSession(c.Request.Context(), claims.SessionID, user.ID, user.CompanyID, auth.HashRefreshToken(refreshCookie), time.Now().Add(idleTimeout))
		if err != nil {
			logInternalError(c, "validate session", err)
			writeError(c, http.StatusInternalServerError, "internal_error", "Unable to validate session")
			c.Abort()
			return
		}
		if !active {
			clearRefreshCookie(c, constants.RefreshCookieName, secureCookie, "/api/v1/auth")
			writeError(c, http.StatusUnauthorized, "session_expired", "Session expired")
			c.Abort()
			return
		}
		if cookieMatches {
			renewRefreshCookie(c, constants.RefreshCookieName, refreshCookie, idleTimeout, secureCookie, "/api/v1/auth")
		}
		permissions, err := repository.ListPermissions(c.Request.Context(), user.ID, user.CompanyID)
		if err != nil {
			logInternalError(c, "load permissions", err)
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

func withTenantSessionFromAccessToken(jwtService *auth.JWTService, next gin.HandlerFunc) gin.HandlerFunc {
	if jwtService == nil {
		return next
	}
	return func(c *gin.Context) {
		if token := bearerToken(c.GetHeader("Authorization")); token != "" {
			if claims, err := jwtService.ValidateAccess(token); err == nil {
				setRequestSessionID(c, claims.SessionID)
			}
		}
		next(c)
	}
}

func renewRefreshCookie(c *gin.Context, name, token string, idleTimeout time.Duration, secure bool, legacyPath string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, token, int(idleTimeout.Seconds()), "/api/v1", "", secure, true)
	// Remove cookies issued by older builds with a narrower path.
	c.SetCookie(name, "", -1, legacyPath, "", secure, true)
}

func clearRefreshCookie(c *gin.Context, name string, secure bool, legacyPath string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, "", -1, "/api/v1", "", secure, true)
	c.SetCookie(name, "", -1, legacyPath, "", secure, true)
}

func CurrentActor(c *gin.Context) (interface{}, bool) {
	value, exists := c.Get(constants.ActorContextKey)
	return value, exists
}
