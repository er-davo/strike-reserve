package service

import (
	"context"
	"errors"

	"booking-service/internal/models"
	"booking-service/internal/repository"
	"booking-service/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo         UserRepository
	passwordCost int
}

func NewUserService(repo UserRepository, passwordCost int) *UserService {
	if passwordCost == 0 {
		passwordCost = bcrypt.DefaultCost
	}
	return &UserService{
		repo:         repo,
		passwordCost: passwordCost,
	}
}

func (s *UserService) Register(ctx context.Context, uRegister models.UserRegister) (*models.User, error) {
	l := logger.FromContext(ctx)

	l.Info("attempting to register new user", zap.String("email", uRegister.Email))

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(uRegister.Password), s.passwordCost)
	if err != nil {
		l.Error("failed to hash password", zap.Error(err), zap.String("email", uRegister.Email))
		return nil, err
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        uRegister.Email,
		PasswordHash: ptr(string(hashedPassword)),
		Role:         models.RoleUser,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		l.Warn("failed to create user in repository",
			zap.Error(err),
			zap.String("email", uRegister.Email),
		)
		return nil, err
	}

	l.Info("user successfully registered",
		zap.String("user_id", user.ID.String()),
		zap.String("email", user.Email),
	)
	return user, nil
}

func (s *UserService) Login(ctx context.Context, uLogin models.UserLogin) (*models.User, error) {
	l := logger.FromContext(ctx)
	l.Debug("login attempt", zap.String("email", uLogin.Email))

	user, err := s.repo.GetByEmail(ctx, uLogin.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			l.Info("login failed: user not found", zap.String("email", uLogin.Email))
			return nil, ErrInvalidEmailOrPassword
		}
		l.Error("repository error during login", zap.Error(err), zap.String("email", uLogin.Email))
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(uLogin.Password))
	if err != nil {
		l.Info("login failed: invalid password", zap.String("email", uLogin.Email))
		return nil, ErrInvalidEmailOrPassword
	}

	l.Info("user logged in successfully", zap.String("user_id", user.ID.String()))
	return user, nil
}

func (s *UserService) GetOrCreateDummy(ctx context.Context, role models.UserRole) (*models.User, error) {
	l := logger.FromContext(ctx)
	l.Info("creating/fetching dummy user", zap.String("role", string(role)))

	id := uuid.New()
	email := "dummy_" + id.String() + "@test.com"
	user := &models.User{
		ID:    id,
		Email: email,
		Role:  role,
	}

	err := s.repo.GetOrCreateForDummy(ctx, user)
	if err != nil {
		l.Error(
			"failed to get or create dummy user",
			zap.Error(err),
			zap.String("role", string(role)),
		)
		return nil, err
	}

	l.Debug("dummy user ready", zap.String("user_id", user.ID.String()))
	return user, nil
}

func ptr[T any](v T) *T {
	return &v
}
