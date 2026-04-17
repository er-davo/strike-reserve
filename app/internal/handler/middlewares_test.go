package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"booking-service/internal/handler"
	"booking-service/internal/mocks"
	"booking-service/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestJWTMiddleware(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockTokenValidator(ctrl)
	e := echo.New()

	next := func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}

	mw := handler.JWTMiddleware(mockAuth)

	t.Run("Success - Valid Token", func(t *testing.T) {
		token := "valid-token"
		userID := uuid.New()
		claims := &models.Claims{UserID: userID.String()}

		mockAuth.EXPECT().ValidateToken(gomock.Any(), token).Return(claims, nil)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := mw(next)(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		ctxValue := c.Request().Context().Value(handler.UserKey)
		assert.NotNil(t, ctxValue, "user_info should be present in context")

		resultClaims, ok := ctxValue.(*models.Claims)
		assert.True(t, ok, "context value should be of type *models.Claims")
		assert.Equal(t, userID.String(), resultClaims.UserID)
	})

	t.Run("Failure - Invalid Token Format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "InvalidFormat token")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := mw(next)(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Failure - Validator Returns Error", func(t *testing.T) {
		token := "bad-token"
		mockAuth.EXPECT().ValidateToken(gomock.Any(), token).Return(nil, errors.New("database down"))

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := mw(next)(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Skip - No Authorization Header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := mw(next)(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code, "should proceed without claims if no header")

		ctxValue := c.Request().Context().Value(handler.UserKey)
		assert.Nil(t, ctxValue)
	})
}
