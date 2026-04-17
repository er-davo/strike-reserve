package handler

import (
	"context"
	"net/http"
	"strings"

	"booking-service/internal/api"
	"booking-service/pkg/logger"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type contextKey string

const UserKey contextKey = "user_info"

func JWTMiddleware(authSrv TokenValidator) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return next(c)
			}

			ctx := c.Request().Context()

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid auth header"})
			}

			claims, err := authSrv.ValidateToken(ctx, parts[1])
			if err != nil {
				c.Logger().Error(err)
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			}

			ctx = context.WithValue(ctx, UserKey, claims)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

func LoggerMiddleware(baseLogger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			reqID := req.Header.Get(echo.HeaderXRequestID)
			if reqID == "" {
				reqID = c.Response().Header().Get(echo.HeaderXRequestID)
			}

			fields := []zap.Field{
				zap.String("request_id", reqID),
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
			}

			enrichedLogger := baseLogger.With(fields...)

			newCtx := logger.ToContext(ctx, enrichedLogger)
			c.SetRequest(req.WithContext(newCtx))

			return next(c)
		}
	}
}

func StrictJWTMiddleware(authSrv TokenValidator) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		return func(c echo.Context, request interface{}) (response interface{}, err error) {
			req := c.Request()
			ctx := c.Request().Context()

			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return next(c, request)
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return nil, echo.NewHTTPError(http.StatusUnauthorized, "invalid auth header")
			}

			claims, err := authSrv.ValidateToken(ctx, parts[1])
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			c.SetRequest(req.WithContext(context.WithValue(ctx, UserKey, claims)))

			return next(c, request)
		}
	}
}

func StrictLoggerMiddleware(baseLogger *zap.Logger) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		return func(c echo.Context, request interface{}) (response interface{}, err error) {
			var method, path, reqID string

			req := c.Request()
			ctx := c.Request().Context()

			r := req
			method = r.Method
			path = r.URL.Path
			reqID = r.Header.Get(echo.HeaderXRequestID)

			log := baseLogger.With(
				zap.String("request_id", reqID),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("operation", operationID),
			)

			ctx = logger.ToContext(ctx, log)

			resp, err := next(c, request)

			return resp, err
		}
	}
}
