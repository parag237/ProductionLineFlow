package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	categoryModule "productionlineflow-api/internal/categories"
	itemsModule "productionlineflow-api/internal/items"
	operationsModule "productionlineflow-api/internal/operations"

	"github.com/gin-gonic/gin"
)

func TestRouterSupportsRootAndVersionedTenantLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(nil)

	for _, path := range []string{"/auth/login", "/api/v1/auth/login"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, response.Code)
			}
		})
	}
}

func TestRouterRegistersOperationLogRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(nil, &Dependencies{OperationLogs: operationsModule.NewService(nil)})

	for _, target := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/operation-logs/options"},
		{http.MethodGet, "/api/v1/operation-logs?warehouse_id=all&work_date=2026-10-04"},
		{http.MethodPost, "/api/v1/operation-logs"},
	} {
		t.Run(target.method+" "+target.path, func(t *testing.T) {
			request := httptest.NewRequest(target.method, target.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected registered handler status %d, got %d", http.StatusUnauthorized, response.Code)
			}
		})
	}
}

func TestRouterHealthChecksAtRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(nil)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, "/", nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
			}
		})
	}
}

func TestRouterRegistersItemAndCategoryRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(nil, &Dependencies{
		Items:      itemsModule.NewService(nil),
		Categories: categoryModule.NewService(nil),
	})

	for _, path := range []string{"/api/v1/items", "/api/v1/categories", "/api/v1/production/runs"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			expectedStatus := http.StatusUnauthorized
			if path == "/api/v1/production/runs" {
				expectedStatus = http.StatusNotFound
			}
			if response.Code != expectedStatus {
				t.Fatalf("expected status %d, got %d", expectedStatus, response.Code)
			}
		})
	}
}
