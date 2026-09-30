package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/company"
	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"

	"github.com/gin-gonic/gin"
)

type PlatformHandler struct {
	service        *auth.PlatformService
	companyService *company.Service
	secureCookie   bool
}

func NewPlatformHandler(service *auth.PlatformService, companyService *company.Service, secureCookie bool) *PlatformHandler {
	return &PlatformHandler{service: service, companyService: companyService, secureCookie: secureCookie}
}

type platformLoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type platformSessionResponse struct {
	AccessToken  string               `json:"access_token"`
	RefreshToken string               `json:"refresh_token"`
	SessionID    string               `json:"session_id"`
	TokenType    string               `json:"token_type"`
	ExpiresIn    int                  `json:"expires_in"`
	IdleTimeout  int                  `json:"idle_timeout_seconds"`
	User         platformUserResponse `json:"user"`
	Permissions  []string             `json:"permissions"`
}

type platformUserResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (h *PlatformHandler) Login(c *gin.Context) {
	var request platformLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid platform login request")
		return
	}
	session, err := h.service.Login(c.Request.Context(), auth.PlatformLoginInput{Email: request.Email, Password: request.Password, UserAgent: c.GetHeader("User-Agent")})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidPlatformCredentials) {
			writeError(c, http.StatusUnauthorized, "invalid_platform_credentials", "Invalid credentials")
			return
		}
		logInternalError(c, "platform sign in", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to sign in")
		return
	}
	c.JSON(http.StatusOK, makePlatformSessionResponse(session))
}

func (h *PlatformHandler) Refresh(c *gin.Context) {
	rawToken := c.GetHeader("X-Refresh-Token")
	session, err := h.service.Refresh(c.Request.Context(), rawToken, c.GetHeader("User-Agent"))
	if err != nil {
		code := "invalid_platform_refresh_token"
		if errors.Is(err, auth.ErrPlatformRefreshReused) {
			code = "platform_refresh_token_reused"
		}
		writeError(c, http.StatusUnauthorized, code, "Platform session expired")
		return
	}
	c.JSON(http.StatusOK, makePlatformSessionResponse(session))
}

func (h *PlatformHandler) Logout(c *gin.Context) {
	rawToken := c.GetHeader("X-Refresh-Token")
	if err := h.service.Logout(c.Request.Context(), rawToken); err != nil {
		logInternalError(c, "platform sign out", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to sign out")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PlatformHandler) Me(c *gin.Context) {
	user, ok := c.Get(constants.PlatformUserContextKey)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Platform authentication required")
		return
	}
	permissions, _ := c.Get(constants.PlatformPermissionsContextKey)
	c.JSON(http.StatusOK, gin.H{"user": makePlatformUserResponse(user.(auth.PlatformUser)), "permissions": permissions})
}

func (h *PlatformHandler) CreateCompany(c *gin.Context) {
	var request struct {
		Name       string `json:"name" binding:"required"`
		Slug       string `json:"slug" binding:"required"`
		SuperAdmin struct {
			Name     string `json:"name" binding:"required"`
			Email    string `json:"email" binding:"required"`
			Password string `json:"password" binding:"required"`
		} `json:"super_admin" binding:"required"`
	}
	value, ok := c.Get(constants.PlatformActorContextKey)
	actor, valid := value.(rbac.PlatformActor)
	if !ok || !valid {
		writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
		return
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid company request")
		return
	}
	if h.companyService == nil {
		writeError(c, http.StatusServiceUnavailable, "onboarding_unavailable", "Company onboarding is unavailable")
		return
	}
	result, err := h.companyService.Create(c.Request.Context(), actor, company.Input{Name: request.Name, Slug: request.Slug, SuperAdminName: request.SuperAdmin.Name, SuperAdminEmail: request.SuperAdmin.Email, SuperAdminPassword: request.SuperAdmin.Password})
	if err != nil {
		if errors.Is(err, company.ErrInvalidInput) {
			writeError(c, http.StatusBadRequest, "invalid_request", "Invalid company request")
			return
		}
		if errors.Is(err, company.ErrForbidden) {
			writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
			return
		}
		if errors.Is(err, company.ErrDuplicateSlug) {
			writeError(c, http.StatusConflict, "duplicate_company_slug", "Company slug already exists")
			return
		}
		if errors.Is(err, company.ErrDuplicateAdminEmail) {
			writeError(c, http.StatusConflict, "duplicate_super_admin_email", "Super Admin email already exists for this company")
			return
		}
		logInternalError(c, "create company", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to create company")
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *PlatformHandler) ListCompanies(c *gin.Context) {
	actor, ok := platformActor(c)
	if !ok {
		writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
		return
	}
	companies, err := h.companyService.List(c.Request.Context(), actor)
	if err != nil {
		h.writeCompanyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"companies": companies})
}

func (h *PlatformHandler) UpdateCompany(c *gin.Context) {
	actor, ok := platformActor(c)
	if !ok {
		writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid company id")
		return
	}
	var request struct {
		Name string `json:"name" binding:"required"`
		Slug string `json:"slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid company request")
		return
	}
	item, err := h.companyService.Update(c.Request.Context(), actor, id, company.UpdateInput{Name: request.Name, Slug: request.Slug})
	if err != nil {
		h.writeCompanyError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *PlatformHandler) SuspendCompany(c *gin.Context) {
	actor, ok := platformActor(c)
	if !ok {
		writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid company id")
		return
	}
	item, err := h.companyService.Suspend(c.Request.Context(), actor, id)
	if err != nil {
		h.writeCompanyError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *PlatformHandler) ReactivateCompany(c *gin.Context) {
	actor, ok := platformActor(c)
	if !ok {
		writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid company id")
		return
	}
	item, err := h.companyService.Reactivate(c.Request.Context(), actor, id)
	if err != nil {
		h.writeCompanyError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func platformActor(c *gin.Context) (rbac.PlatformActor, bool) {
	value, ok := c.Get(constants.PlatformActorContextKey)
	actor, valid := value.(rbac.PlatformActor)
	return actor, ok && valid
}

func (h *PlatformHandler) writeCompanyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, company.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
	case errors.Is(err, company.ErrInvalidInput):
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid company request")
	case errors.Is(err, company.ErrNotFound):
		writeError(c, http.StatusNotFound, "company_not_found", "Company not found")
	case errors.Is(err, company.ErrDuplicateSlug):
		writeError(c, http.StatusConflict, "duplicate_company_slug", "Company slug already exists")
	default:
		logInternalError(c, "manage company", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to manage company")
	}
}

func makePlatformSessionResponse(session auth.PlatformSession) platformSessionResponse {
	return platformSessionResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken, SessionID: session.SessionID, TokenType: "Bearer", ExpiresIn: session.ExpiresIn, IdleTimeout: session.IdleTimeout, User: makePlatformUserResponse(session.User), Permissions: session.Permissions}
}

func makePlatformUserResponse(user auth.PlatformUser) platformUserResponse {
	return platformUserResponse{ID: user.ID, Name: user.Name, Email: user.Email}
}

func RequirePlatformAuth(jwtService *auth.JWTService, repository auth.PlatformRepository, idleTimeout time.Duration, secureCookie bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		claims, err := jwtService.ValidatePlatformAccess(token)
		if token == "" || err != nil {
			writeError(c, http.StatusUnauthorized, "unauthorized", "Platform authentication required")
			c.Abort()
			return
		}
		user, err := repository.FindPlatformUserByID(c.Request.Context(), claims.PlatformUserID)
		if err != nil || !user.IsActive {
			writeError(c, http.StatusUnauthorized, "unauthorized", "Platform authentication required")
			c.Abort()
			return
		}
		if user.PermVersion != claims.PermVersion {
			writeError(c, http.StatusUnauthorized, "permissions_changed", "Permissions changed; refresh your session")
			c.Abort()
			return
		}
		refreshCookie, _ := c.Cookie(constants.PlatformRefreshCookieName)
		active, cookieMatches, err := repository.TouchPlatformSession(c.Request.Context(), claims.SessionID, user.ID, auth.HashRefreshToken(refreshCookie), time.Now().Add(idleTimeout))
		if err != nil {
			logInternalError(c, "validate platform session", err)
			writeError(c, http.StatusInternalServerError, "internal_error", "Unable to validate platform session")
			c.Abort()
			return
		}
		if !active {
			clearRefreshCookie(c, constants.PlatformRefreshCookieName, secureCookie, "/api/v1/platform/auth")
			writeError(c, http.StatusUnauthorized, "session_expired", "Platform session expired")
			c.Abort()
			return
		}
		if cookieMatches {
			renewRefreshCookie(c, constants.PlatformRefreshCookieName, refreshCookie, idleTimeout, secureCookie, "/api/v1/platform/auth")
		}
		permissions, err := repository.ListPlatformPermissions(c.Request.Context(), user.ID)
		if err != nil {
			logInternalError(c, "load platform permissions", err)
			writeError(c, http.StatusInternalServerError, "internal_error", "Unable to load platform permissions")
			c.Abort()
			return
		}
		actor := rbac.PlatformActor{UserID: user.ID, Permissions: permissions}
		c.Set(constants.PlatformUserContextKey, user)
		c.Set(constants.PlatformPermissionsContextKey, permissions)
		c.Set(constants.PlatformActorContextKey, actor)
		c.Next()
	}
}

func RequirePlatformPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(constants.PlatformActorContextKey)
		actor, valid := value.(rbac.PlatformActor)
		if !ok || !valid || !actor.Can(permission) {
			writeError(c, http.StatusForbidden, "forbidden", "Platform permission required")
			c.Abort()
			return
		}
		c.Next()
	}
}
