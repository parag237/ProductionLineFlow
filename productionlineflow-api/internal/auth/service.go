package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"productionlineflow-api/internal/rbac"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRefreshInvalid     = errors.New("invalid refresh token")
	ErrRefreshReused      = errors.New("refresh token reuse detected")
)

type User struct {
	ID           int64
	CompanyID    int64
	CompanySlug  string
	Name         string
	Email        string
	PasswordHash string
	IsActive     bool
	PermVersion  int
	SessionID    string
}

type Permission struct {
	Key         string
	WarehouseID *int64
	RoleSlug    string
	RoleScope   string
}

type Repository interface {
	FindUserByLogin(ctx context.Context, companySlug, email string) (User, error)
	FindUserByID(ctx context.Context, userID, companyID int64) (User, error)
	ListPermissions(ctx context.Context, userID, companyID int64) ([]Permission, error)
	CreateRefreshToken(ctx context.Context, userID, companyID int64, sessionID, hash string, expiresAt time.Time, userAgent string) error
	RotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time, userAgent string) (User, error)
	RevokeRefreshToken(ctx context.Context, hash string) error
	TouchTenantSession(ctx context.Context, sessionID string, userID, companyID int64, refreshHash string, expiresAt time.Time) (bool, bool, error)
}

type Service struct {
	repository  Repository
	jwt         *JWTService
	refreshTTL  time.Duration
	minPassword int
}

func NewService(repository Repository, jwt *JWTService, refreshTTL time.Duration, minPassword int) *Service {
	return &Service{repository: repository, jwt: jwt, refreshTTL: refreshTTL, minPassword: minPassword}
}

func (s *Service) RefreshTTL() time.Duration {
	return s.refreshTTL
}

type LoginInput struct {
	CompanySlug string
	Email       string
	Password    string
	UserAgent   string
}

type Session struct {
	AccessToken  string
	RefreshToken string
	SessionID    string
	ExpiresIn    int
	IdleTimeout  int
	User         User
	Permissions  []Permission
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Session, error) {
	if strings.TrimSpace(input.CompanySlug) == "" || strings.TrimSpace(input.Email) == "" || input.Password == "" {
		return Session{}, ErrInvalidCredentials
	}

	user, err := s.repository.FindUserByLogin(ctx, strings.TrimSpace(input.CompanySlug), strings.TrimSpace(input.Email))
	if err != nil || !user.IsActive || !VerifyPassword(input.Password, user.PasswordHash) {
		return Session{}, ErrInvalidCredentials
	}

	sessionID, err := newSessionID()
	if err != nil {
		return Session{}, fmt.Errorf("create session id: %w", err)
	}
	accessToken, err := s.jwt.SignAccess(user.ID, user.CompanyID, user.PermVersion, sessionID)
	if err != nil {
		return Session{}, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, refreshHash, err := newRefreshToken()
	if err != nil {
		return Session{}, fmt.Errorf("create refresh token: %w", err)
	}
	if err := s.repository.CreateRefreshToken(ctx, user.ID, user.CompanyID, sessionID, refreshHash, time.Now().Add(s.refreshTTL), input.UserAgent); err != nil {
		return Session{}, fmt.Errorf("store refresh token: %w", err)
	}

	permissions, err := s.repository.ListPermissions(ctx, user.ID, user.CompanyID)
	if err != nil {
		return Session{}, fmt.Errorf("load permissions: %w", err)
	}

	return Session{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
		ExpiresIn:    int(s.jwt.TTL().Seconds()),
		IdleTimeout:  int(s.refreshTTL.Seconds()),
		User:         user,
		Permissions:  permissions,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawToken, userAgent string) (Session, error) {
	if rawToken == "" {
		return Session{}, ErrRefreshInvalid
	}
	newToken, newHash, err := newRefreshToken()
	if err != nil {
		return Session{}, fmt.Errorf("create refresh token: %w", err)
	}

	user, err := s.repository.RotateRefreshToken(ctx, hashToken(rawToken), newHash, time.Now().Add(s.refreshTTL), userAgent)
	if err != nil {
		return Session{}, err
	}
	accessToken, err := s.jwt.SignAccess(user.ID, user.CompanyID, user.PermVersion, user.SessionID)
	if err != nil {
		return Session{}, fmt.Errorf("sign access token: %w", err)
	}
	permissions, err := s.repository.ListPermissions(ctx, user.ID, user.CompanyID)
	if err != nil {
		return Session{}, fmt.Errorf("load permissions: %w", err)
	}

	return Session{AccessToken: accessToken, RefreshToken: newToken, SessionID: user.SessionID, ExpiresIn: int(s.jwt.TTL().Seconds()), IdleTimeout: int(s.refreshTTL.Seconds()), User: user, Permissions: permissions}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.repository.RevokeRefreshToken(ctx, hashToken(rawToken))
}

func PermissionsForActor(permissions []Permission) rbac.Actor {
	actor := rbac.Actor{}
	for _, permission := range permissions {
		actor.Assignments = append(actor.Assignments, rbac.Assignment{Permission: permission.Key, WarehouseID: permission.WarehouseID})
	}
	return actor
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func HashRefreshToken(token string) string { return hashToken(token) }

func newSessionID() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func newRefreshToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}
