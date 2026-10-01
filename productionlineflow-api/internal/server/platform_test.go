package server

import (
	"testing"

	"productionlineflow-api/internal/auth"
)

func TestMakePlatformSessionResponseUsesEmptyPermissionsArray(t *testing.T) {
	response := makePlatformSessionResponse(auth.PlatformSession{})
	if response.Permissions == nil {
		t.Fatal("expected permissions to be an empty array, got nil")
	}
	if len(response.Permissions) != 0 {
		t.Fatalf("expected no permissions, got %v", response.Permissions)
	}
}