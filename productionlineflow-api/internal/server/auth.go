package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/company"
	"productionlineflow-api/internal/constants"
	itemsModule "productionlineflow-api/internal/items"
	"productionlineflow-api/internal/people"
	"productionlineflow-api/internal/rbac"
	warehouseModule "productionlineflow-api/internal/warehouse"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Auth               *auth.Service
	PlatformAuth       *auth.PlatformService
	JWT                *auth.JWTService
	Repository         auth.Repository
	PlatformRepository auth.PlatformRepository
	Company            *company.Service
	Warehouse          *warehouseModule.Service
	People             *people.Service
	Items              *itemsModule.Service
	Production         *itemsModule.ProductionService
	SecureCookie       bool
}

type AuthHandler struct {
	service      *auth.Service
	secureCookie bool
}

func NewAuthHandler(service *auth.Service, secureCookie bool) *AuthHandler {
	return &AuthHandler{service: service, secureCookie: secureCookie}
}

type loginRequest struct {
	CompanySlug string `json:"company_slug" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

type eventlessUser struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	CompanyID   int64  `json:"company_id"`
	CompanySlug string `json:"company_slug"`
}

type sessionResponse struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	SessionID    string             `json:"session_id"`
	TokenType    string             `json:"token_type"`
	ExpiresIn    int                `json:"expires_in"`
	IdleTimeout  int                `json:"idle_timeout_seconds"`
	User         eventlessUser      `json:"user"`
	Permissions  permissionResponse `json:"permissions"`
}

type permissionResponse struct {
	Company    []string            `json:"company"`
	Warehouses map[string][]string `json:"warehouses"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid login request")
		return
	}
	session, err := h.service.Login(c.Request.Context(), auth.LoginInput{
		CompanySlug: request.CompanySlug,
		Email:       request.Email,
		Password:    request.Password,
		UserAgent:   c.GetHeader("User-Agent"),
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(c, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
			return
		}
		logInternalError(c, "sign in", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to sign in")
		return
	}
	setRequestSessionID(c, session.SessionID)
	c.JSON(http.StatusOK, makeSessionResponse(session))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	rawToken := c.GetHeader("X-Refresh-Token")
	session, err := h.service.Refresh(c.Request.Context(), rawToken, c.GetHeader("User-Agent"))
	if err != nil {
		code := "invalid_refresh_token"
		if errors.Is(err, auth.ErrRefreshReused) {
			code = "refresh_token_reused"
		}
		writeError(c, http.StatusUnauthorized, code, "Session expired")
		return
	}
	setRequestSessionID(c, session.SessionID)
	c.JSON(http.StatusOK, makeSessionResponse(session))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	rawToken := c.GetHeader("X-Refresh-Token")
	if err := h.service.Logout(c.Request.Context(), rawToken); err != nil {
		logInternalError(c, "sign out", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to sign out")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	permissions, _ := c.Get(constants.PermissionsContextKey)
	permissionList, _ := permissions.([]auth.Permission)
	c.JSON(http.StatusOK, gin.H{
		"user":         makeUserResponse(user),
		"permissions":  makePermissionResponse(permissionList),
		"perm_version": user.PermVersion,
	})
}

func makeSessionResponse(session auth.Session) sessionResponse {
	return sessionResponse{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		SessionID:    session.SessionID,
		TokenType:    "Bearer",
		ExpiresIn:    session.ExpiresIn,
		IdleTimeout:  session.IdleTimeout,
		User:         makeUserResponse(session.User),
		Permissions:  makePermissionResponse(session.Permissions),
	}
}

func makeUserResponse(user auth.User) eventlessUser {
	return eventlessUser{ID: user.ID, Name: user.Name, Email: user.Email, CompanyID: user.CompanyID, CompanySlug: user.CompanySlug}
}

func makePermissionResponse(permissions []auth.Permission) permissionResponse {
	response := permissionResponse{Company: []string{}, Warehouses: map[string][]string{}}
	seenCompany := map[string]bool{}
	seenWarehouse := map[string]map[string]bool{}
	for _, permission := range permissions {
		if permission.WarehouseID == nil {
			if !seenCompany[permission.Key] {
				response.Company = append(response.Company, permission.Key)
				seenCompany[permission.Key] = true
			}
			continue
		}
		warehouseID := strconv.FormatInt(*permission.WarehouseID, 10)
		if seenWarehouse[warehouseID] == nil {
			seenWarehouse[warehouseID] = map[string]bool{}
		}
		if !seenWarehouse[warehouseID][permission.Key] {
			response.Warehouses[warehouseID] = append(response.Warehouses[warehouseID], permission.Key)
			seenWarehouse[warehouseID][permission.Key] = true
		}
	}
	return response
}

func currentUser(c *gin.Context) (auth.User, bool) {
	value, exists := c.Get(constants.UserContextKey)
	if !exists {
		return auth.User{}, false
	}
	user, ok := value.(auth.User)
	return user, ok
}

func writeError(c *gin.Context, status int, code, message string) {
	c.Set(errorCodeKey, code)
	errorBody := gin.H{"code": code, "message": message}
	if requestID, exists := c.Get(requestIDKey); exists {
		errorBody["request_id"] = requestID
	}
	c.JSON(status, gin.H{"error": errorBody})
}

func actorFromPermissions(user auth.User, permissions []auth.Permission) rbac.Actor {
	actor := auth.PermissionsForActor(permissions)
	actor.UserID = user.ID
	actor.CompanyID = user.CompanyID
	return actor
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
