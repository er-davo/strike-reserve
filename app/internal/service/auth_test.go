package service_test

import (
	"context"
	"testing"
	"time"

	"booking-service/internal/models"
	"booking-service/internal/service"
	"booking-service/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestAuthService_GenerateToken(t *testing.T) {
	secret := "super-secret-key"
	duration := time.Hour
	svc := service.NewAuthService(secret, duration)

	user := &models.User{
		ID:   uuid.New(),
		Role: models.RoleAdmin,
	}

	t.Run("Success: generate and parse token", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		tokenString, err := svc.GenerateToken(ctx, user)
		require.NoError(t, err)
		require.NotEmpty(t, tokenString)

		parsedClaims := &models.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, parsedClaims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		require.NoError(t, err)
		assert.True(t, token.Valid)

		assert.Equal(t, user.ID.String(), parsedClaims.UserID)
		assert.Equal(t, user.Role, parsedClaims.Role)
		assert.Equal(t, user.ID.String(), parsedClaims.Subject)

		assert.WithinDuration(t, time.Now().Add(duration), parsedClaims.ExpiresAt.Time, time.Second*5)
	})

	t.Run("Security: invalid secret fails parsing", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		tokenString, err := svc.GenerateToken(ctx, user)
		require.NoError(t, err)

		wrongSecret := "another-secret"
		_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(wrongSecret), nil
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature is invalid")
	})
}

func TestAuthService_ValidateToken(t *testing.T) {
	svc := service.NewAuthService("secret", time.Hour)
	user := &models.User{ID: uuid.New(), Role: models.RoleUser}

	token, _ := svc.GenerateToken(context.Background(), user)

	t.Run("Valid token", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		claims, err := svc.ValidateToken(ctx, token)
		assert.NoError(t, err)
		assert.Equal(t, user.ID.String(), claims.UserID)
	})

	t.Run("Invalid algorithm (none)", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		noneToken := "eyJhbGciOiJub25lIn0.eyJzdWIiOiIxMjMifQ."
		_, err := svc.ValidateToken(ctx, noneToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected signing method")
	})
}
