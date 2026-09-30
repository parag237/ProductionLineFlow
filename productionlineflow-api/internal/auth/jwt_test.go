package auth

import (
	"strings"
	"testing"
	"time"
)

func TestJWTServiceValidatesTypedAccessClaims(t *testing.T) {
	service := NewJWTService("test-secret", time.Minute, "test-issuer")
	token, err := service.SignAccess(12, 34, 2, "test-session")
	if err != nil {
		t.Fatalf("SignAccess returned error: %v", err)
	}

	claims, err := service.ValidateAccess(token)
	if err != nil {
		t.Fatalf("ValidateAccess returned error: %v", err)
	}
	if claims.UserID != 12 || claims.CompanyID != 34 || claims.PermVersion != 2 || claims.SessionID != "test-session" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestJWTServiceRequiresSessionID(t *testing.T) {
	service := NewJWTService("test-secret", time.Minute, "test-issuer")
	tenant, err := service.SignAccess(1, 2, 1, "")
	if err != nil {
		t.Fatalf("SignAccess returned error: %v", err)
	}
	if _, err := service.ValidateAccess(tenant); err == nil {
		t.Fatal("tenant token without session ID was accepted")
	}
	platform, err := service.SignPlatformAccess(1, 1, "")
	if err != nil {
		t.Fatalf("SignPlatformAccess returned error: %v", err)
	}
	if _, err := service.ValidatePlatformAccess(platform); err == nil {
		t.Fatal("platform token without session ID was accepted")
	}
}

func TestJWTServiceRejectsWrongIssuerAndMalformedToken(t *testing.T) {
	service := NewJWTService("test-secret", time.Minute, "test-issuer")
	other := NewJWTService("test-secret", time.Minute, "other-issuer")
	token, err := other.SignAccess(1, 2, 1, "other-session")
	if err != nil {
		t.Fatalf("SignAccess returned error: %v", err)
	}
	if _, err := service.ValidateAccess(token); err == nil {
		t.Fatal("expected wrong issuer to fail")
	}
	if _, err := service.ValidateAccess(strings.TrimSuffix(token, "x")); err == nil {
		t.Fatal("expected malformed token to fail")
	}
}
