package auth

import (
	"context"
	"testing"
	"time"
)

type tenantSessionRepo struct {
	Repository
	user        User
	sessionID   string
	refreshHash string
	expiresAt   time.Time
}

func (r *tenantSessionRepo) FindUserByLogin(context.Context, string, string) (User, error) {
	return r.user, nil
}
func (r *tenantSessionRepo) ListPermissions(context.Context, int64, int64) ([]Permission, error) {
	return []Permission{}, nil
}
func (r *tenantSessionRepo) CreateRefreshToken(_ context.Context, _, _ int64, sessionID, hash string, expiresAt time.Time, _ string) error {
	r.sessionID, r.refreshHash, r.expiresAt = sessionID, hash, expiresAt
	return nil
}
func (r *tenantSessionRepo) RotateRefreshToken(_ context.Context, _ string, hash string, expiresAt time.Time, _ string) (User, error) {
	r.refreshHash, r.expiresAt = hash, expiresAt
	r.user.SessionID = r.sessionID
	return r.user, nil
}
func (r *tenantSessionRepo) RevokeRefreshToken(context.Context, string) error { return nil }
func (r *tenantSessionRepo) TouchTenantSession(context.Context, string, int64, int64, string, time.Time) (bool, bool, error) {
	return true, true, nil
}

type platformSessionRepo struct {
	PlatformRepository
	user        PlatformUser
	sessionID   string
	refreshHash string
}

func (r *platformSessionRepo) FindPlatformUserByLogin(context.Context, string) (PlatformUser, error) {
	return r.user, nil
}
func (r *platformSessionRepo) ListPlatformPermissions(context.Context, int64) ([]string, error) {
	return []string{}, nil
}
func (r *platformSessionRepo) CreatePlatformRefreshToken(_ context.Context, _ int64, sessionID, hash string, _ time.Time, _ string) error {
	r.sessionID, r.refreshHash = sessionID, hash
	return nil
}
func (r *platformSessionRepo) RotatePlatformRefreshToken(_ context.Context, _ string, hash string, _ time.Time, _ string) (PlatformUser, error) {
	r.refreshHash = hash
	r.user.SessionID = r.sessionID
	return r.user, nil
}
func (r *platformSessionRepo) RevokePlatformRefreshToken(context.Context, string) error { return nil }
func (r *platformSessionRepo) TouchPlatformSession(context.Context, string, int64, string, time.Time) (bool, bool, error) {
	return true, true, nil
}

func TestTenantLoginAndRefreshPreserveSlidingSession(t *testing.T) {
	passwordHash, err := HashPassword("test-password")
	if err != nil {
		t.Fatal(err)
	}
	repo := &tenantSessionRepo{user: User{ID: 5, CompanyID: 9, CompanySlug: "acme", Email: "user@acme.test", PasswordHash: passwordHash, IsActive: true, PermVersion: 2}}
	jwt := NewJWTService("test-secret", 15*time.Minute, "test-issuer")
	service := NewService(repo, jwt, 7*24*time.Hour, 8)
	login, err := service.Login(context.Background(), LoginInput{CompanySlug: "acme", Email: "user@acme.test", Password: "test-password"})
	if err != nil {
		t.Fatal(err)
	}
	if login.SessionID == "" || repo.sessionID != login.SessionID || login.IdleTimeout != int((7*24*time.Hour).Seconds()) {
		t.Fatalf("unexpected login session: %#v", login)
	}
	claims, err := jwt.ValidateAccess(login.AccessToken)
	if err != nil || claims.SessionID != login.SessionID {
		t.Fatalf("access token session mismatch: claims=%#v err=%v", claims, err)
	}
	refreshed, err := service.Refresh(context.Background(), login.RefreshToken, "")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.SessionID != login.SessionID || refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh did not rotate credentials in the same session")
	}
	refreshedClaims, err := jwt.ValidateAccess(refreshed.AccessToken)
	if err != nil || refreshedClaims.SessionID != login.SessionID {
		t.Fatalf("refreshed token session mismatch: claims=%#v err=%v", refreshedClaims, err)
	}
}

func TestPlatformLoginAndRefreshPreserveSlidingSession(t *testing.T) {
	passwordHash, err := HashPassword("test-password")
	if err != nil {
		t.Fatal(err)
	}
	repo := &platformSessionRepo{user: PlatformUser{ID: 3, Email: "owner@platform.test", PasswordHash: passwordHash, IsActive: true, PermVersion: 1}}
	jwt := NewJWTService("test-secret", 15*time.Minute, "test-issuer")
	service := NewPlatformService(repo, jwt, 7*24*time.Hour)
	login, err := service.Login(context.Background(), PlatformLoginInput{Email: "owner@platform.test", Password: "test-password"})
	if err != nil {
		t.Fatal(err)
	}
	if login.SessionID == "" || repo.sessionID != login.SessionID || login.IdleTimeout != int((7*24*time.Hour).Seconds()) {
		t.Fatalf("unexpected platform session: %#v", login)
	}
	claims, err := jwt.ValidatePlatformAccess(login.AccessToken)
	if err != nil || claims.SessionID != login.SessionID {
		t.Fatalf("platform token session mismatch: claims=%#v err=%v", claims, err)
	}
	refreshed, err := service.Refresh(context.Background(), login.RefreshToken, "")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.SessionID != login.SessionID || refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("platform refresh did not rotate in the same session")
	}
}
