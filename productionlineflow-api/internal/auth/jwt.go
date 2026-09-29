package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTService(secret string, ttl time.Duration) *JWTService {
	return &JWTService{secret: []byte(secret), ttl: ttl}
}

func (s *JWTService) Sign(claims map[string]any) (string, error) {
	payload := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims))
	return payload.SignedString(s.secret)
}

func (s *JWTService) Validate(token string) (map[string]any, error) {
	parsed, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
		return map[string]any(claims), nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

func (s *JWTService) TTL() time.Duration {
	return s.ttl
}
