package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"productionlineflow-api/internal/auth"

	"github.com/gin-gonic/gin"
)

type tenantMiddlewareRepo struct {
	auth.Repository
	user           auth.User
	active         bool
	touchedSession string
	touchedHash    string
}

func (r *tenantMiddlewareRepo) FindUserByID(_ context.Context, userID, companyID int64) (auth.User, error) {
	return r.user, nil
}
func (r *tenantMiddlewareRepo) ListPermissions(context.Context, int64, int64) ([]auth.Permission, error) {
	return []auth.Permission{}, nil
}
func (r *tenantMiddlewareRepo) TouchTenantSession(_ context.Context, sessionID string, _, _ int64, refreshHash string, _ time.Time) (bool, bool, error) {
	r.touchedSession, r.touchedHash = sessionID, refreshHash
	return r.active, true, nil
}

type platformMiddlewareRepo struct {
	auth.PlatformRepository
	user           auth.PlatformUser
	active         bool
	touchedSession string
}

func (r *platformMiddlewareRepo) FindPlatformUserByID(context.Context, int64) (auth.PlatformUser, error) {
	return r.user, nil
}
func (r *platformMiddlewareRepo) ListPlatformPermissions(context.Context, int64) ([]string, error) {
	return []string{}, nil
}
func (r *platformMiddlewareRepo) TouchPlatformSession(_ context.Context, sessionID string, _ int64, _ string, _ time.Time) (bool, bool, error) {
	r.touchedSession = sessionID
	return r.active, true, nil
}

func TestTenantMiddlewareTouchesSessionAndRenewsRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwt := auth.NewJWTService("test-secret", time.Minute, "test-issuer")
	token, err := jwt.SignAccess(4, 8, 1, "tenant-session")
	if err != nil {
		t.Fatal(err)
	}
	repo := &tenantMiddlewareRepo{user: auth.User{ID: 4, CompanyID: 8, IsActive: true, PermVersion: 1}, active: true}
	router := gin.New()
	router.GET("/api/v1/me", RequireTenantAuth(jwt, repo, time.Hour, false), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.AddCookie(&http.Cookie{Name: "productionlineflow_refresh", Value: "raw-refresh"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repo.touchedSession != "tenant-session" || repo.touchedHash != auth.HashRefreshToken("raw-refresh") {
		t.Fatalf("session was not touched with the matching refresh cookie: %#v", repo)
	}
	foundRenewal := false
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "productionlineflow_refresh" && cookie.Path == "/api/v1" && cookie.MaxAge > 0 {
			foundRenewal = true
		}
	}
	if !foundRenewal {
		t.Fatal("sliding refresh cookie was not renewed for the API path")
	}
}

func TestExpiredTenantSessionIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwt := auth.NewJWTService("test-secret", time.Minute, "test-issuer")
	token, err := jwt.SignAccess(4, 8, 1, "expired-session")
	if err != nil {
		t.Fatal(err)
	}
	repo := &tenantMiddlewareRepo{user: auth.User{ID: 4, CompanyID: 8, IsActive: true, PermVersion: 1}, active: false}
	called := false
	router := gin.New()
	router.GET("/api/v1/me", RequireTenantAuth(jwt, repo, time.Hour, false), func(c *gin.Context) { called = true })
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || called {
		t.Fatalf("expired session was allowed: status=%d called=%v", response.Code, called)
	}
}

func TestPlatformMiddlewareTouchesItsOwnSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwt := auth.NewJWTService("test-secret", time.Minute, "test-issuer")
	token, err := jwt.SignPlatformAccess(7, 2, "platform-session")
	if err != nil {
		t.Fatal(err)
	}
	repo := &platformMiddlewareRepo{user: auth.PlatformUser{ID: 7, IsActive: true, PermVersion: 2}, active: true}
	router := gin.New()
	router.GET("/api/v1/platform/me", RequirePlatformAuth(jwt, repo, time.Hour, false), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/api/v1/platform/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.AddCookie(&http.Cookie{Name: "productionlineflow_platform_refresh", Value: "platform-refresh"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || repo.touchedSession != "platform-session" {
		t.Fatalf("platform session not validated/touched: status=%d repo=%#v", response.Code, repo)
	}
}
