package auth

import (
	"fmt"
	"time"

	"productionlineflow-api/internal/constants"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

type AccessClaims struct {
	UserID      int64  `json:"user_id"`
	CompanyID   int64  `json:"company_id"`
	SessionID   string `json:"sid"`
	PermVersion int    `json:"perm_version"`
	TokenType   string `json:"token_type"`
	jwt.RegisteredClaims
}

type PlatformClaims struct {
	PlatformUserID int64  `json:"platform_user_id"`
	SessionID      string `json:"sid"`
	PermVersion    int    `json:"perm_version"`
	TokenType      string `json:"token_type"`
	jwt.RegisteredClaims
}

func NewJWTService(secret string, ttl time.Duration, issuer string) *JWTService {
	return &JWTService{secret: []byte(secret), ttl: ttl, issuer: issuer}
}

func (s *JWTService) SignAccess(userID, companyID int64, permVersion int, sessionID string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		UserID:      userID,
		CompanyID:   companyID,
		SessionID:   sessionID,
		PermVersion: permVersion,
		TokenType:   constants.TenantAccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ID:        fmt.Sprintf("%d-%d", userID, now.UnixNano()),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	payload := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return payload.SignedString(s.secret)
}

func (s *JWTService) SignPlatformAccess(userID int64, permVersion int, sessionID string) (string, error) {
	now := time.Now()
	claims := PlatformClaims{
		PlatformUserID: userID,
		SessionID:      sessionID,
		PermVersion:    permVersion,
		TokenType:      constants.PlatformAccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ID:        fmt.Sprintf("platform-%d-%d", userID, now.UnixNano()),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *JWTService) ValidateAccess(token string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(s.issuer))
	if err != nil {
		return nil, err
	}
	if !parsed.Valid || claims.TokenType != constants.TenantAccessTokenType || claims.SessionID == "" {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func (s *JWTService) ValidatePlatformAccess(token string) (*PlatformClaims, error) {
	claims := &PlatformClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(s.issuer))
	if err != nil {
		return nil, err
	}
	if !parsed.Valid || claims.TokenType != constants.PlatformAccessTokenType || claims.SessionID == "" {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func (s *JWTService) TTL() time.Duration {
	return s.ttl
}
