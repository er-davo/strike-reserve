package service

import (
	"context"
	"fmt"
	"time"

	"booking-service/internal/models"
	"booking-service/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type AuthService struct {
	secretKey     []byte
	tokenDuration time.Duration
}

func NewAuthService(
	secret string,
	tokenDuration time.Duration,
) *AuthService {
	return &AuthService{
		secretKey:     []byte(secret),
		tokenDuration: tokenDuration,
	}
}

func (s *AuthService) GenerateToken(ctx context.Context, user *models.User) (string, error) {
	l := logger.FromContext(ctx)

	claims := models.Claims{
		UserID: user.ID.String(),
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		l.Error("failed to sign jwt token",
			zap.Error(err),
			zap.String("user_id", user.ID.String()),
		)
		return "", err
	}

	return tokenString, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (*models.Claims, error) {
	claims := &models.Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		logger.FromContext(ctx).Warn("invalid token presented")
		return nil, ErrTokenIsInvalid
	}

	return claims, nil
}
