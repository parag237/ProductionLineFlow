package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidPlatformCredentials = errors.New("invalid platform credentials")
	ErrPlatformRefreshInvalid     = errors.New("invalid platform refresh token")
	ErrPlatformRefreshReused      = errors.New("platform refresh token reuse detected")
)

type PlatformUser struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	IsActive     bool
	PermVersion  int
}

type PlatformRepository interface {
	FindPlatformUserByLogin(ctx context.Context, email string) (PlatformUser, error)
	FindPlatformUserByID(ctx context.Context, userID int64) (PlatformUser, error)
	ListPlatformPermissions(ctx context.Context, userID int64) ([]string, error)
	CreatePlatformRefreshToken(ctx context.Context, userID int64, hash string, expiresAt time.Time, userAgent string) error
	RotatePlatformRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time, userAgent string) (PlatformUser, error)
	RevokePlatformRefreshToken(ctx context.Context, hash string) error
}

type PlatformService struct {
	repository PlatformRepository
	jwt        *JWTService
	refreshTTL time.Duration
}

func NewPlatformService(repository PlatformRepository, jwt *JWTService, refreshTTL time.Duration) *PlatformService {
	return &PlatformService{repository: repository, jwt: jwt, refreshTTL: refreshTTL}
}

func (s *PlatformService) RefreshTTL() time.Duration { return s.refreshTTL }

type PlatformLoginInput struct {
	Email     string
	Password  string
	UserAgent string
}

type PlatformSession struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	User         PlatformUser
	Permissions  []string
}

func (s *PlatformService) Login(ctx context.Context, input PlatformLoginInput) (PlatformSession, error) {
	if strings.TrimSpace(input.Email) == "" || input.Password == "" {
		return PlatformSession{}, ErrInvalidPlatformCredentials
	}
	user, err := s.repository.FindPlatformUserByLogin(ctx, strings.TrimSpace(input.Email))
	if err != nil || !user.IsActive || !VerifyPassword(input.Password, user.PasswordHash) {
		return PlatformSession{}, ErrInvalidPlatformCredentials
	}
	access, err := s.jwt.SignPlatformAccess(user.ID, user.PermVersion)
	if err != nil {
		return PlatformSession{}, err
	}
	refresh, refreshHash, err := newRefreshToken()
	if err != nil {
		return PlatformSession{}, err
	}
	if err := s.repository.CreatePlatformRefreshToken(ctx, user.ID, refreshHash, time.Now().Add(s.refreshTTL), input.UserAgent); err != nil {
		return PlatformSession{}, err
	}
	permissions, err := s.repository.ListPlatformPermissions(ctx, user.ID)
	if err != nil {
		return PlatformSession{}, err
	}
	return PlatformSession{AccessToken: access, RefreshToken: refresh, ExpiresIn: int(s.jwt.TTL().Seconds()), User: user, Permissions: permissions}, nil
}

func (s *PlatformService) Refresh(ctx context.Context, rawToken, userAgent string) (PlatformSession, error) {
	if rawToken == "" {
		return PlatformSession{}, ErrPlatformRefreshInvalid
	}
	refresh, refreshHash, err := newRefreshToken()
	if err != nil {
		return PlatformSession{}, err
	}
	user, err := s.repository.RotatePlatformRefreshToken(ctx, hashToken(rawToken), refreshHash, time.Now().Add(s.refreshTTL), userAgent)
	if err != nil {
		return PlatformSession{}, err
	}
	access, err := s.jwt.SignPlatformAccess(user.ID, user.PermVersion)
	if err != nil {
		return PlatformSession{}, err
	}
	permissions, err := s.repository.ListPlatformPermissions(ctx, user.ID)
	if err != nil {
		return PlatformSession{}, err
	}
	return PlatformSession{AccessToken: access, RefreshToken: refresh, ExpiresIn: int(s.jwt.TTL().Seconds()), User: user, Permissions: permissions}, nil
}

func (s *PlatformService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.repository.RevokePlatformRefreshToken(ctx, hashToken(rawToken))
}
