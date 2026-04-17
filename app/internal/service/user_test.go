package service_test

import (
	"context"
	"errors"
	"testing"

	"booking-service/internal/mocks"
	"booking-service/internal/models"
	"booking-service/internal/repository"
	"booking-service/internal/service"
	"booking-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
	"golang.org/x/crypto/bcrypt"
)

func TestUserService_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewUserService(mockRepo, bcrypt.MinCost)

	uRegister := models.UserRegister{
		Email:    "test@example.com",
		Password: "password123",
	}

	t.Run("Success", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		mockRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, u *models.User) error {
				assert.Equal(t, uRegister.Email, u.Email)
				assert.Equal(t, models.RoleUser, u.Role)
				assert.NotNil(t, u.PasswordHash)

				err := bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(uRegister.Password))
				assert.NoError(t, err)
				return nil
			})

		res, err := svc.Register(ctx, uRegister)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Repo Error", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

		res, err := svc.Register(ctx, uRegister)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Bcrypt Cost Error", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		invalidSvc := service.NewUserService(mockRepo, 32)

		res, err := invalidSvc.Register(ctx, uRegister)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestUserService_Login(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewUserService(mockRepo, bcrypt.MinCost)

	email := "user@test.com"
	password := "secret"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	dbUser := &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: ptr(string(hashedPassword)),
	}

	tests := []struct {
		name        string
		loginData   models.UserLogin
		setup       func()
		expectedErr error
	}{
		{
			name:      "Success",
			loginData: models.UserLogin{Email: email, Password: password},
			setup: func() {
				mockRepo.EXPECT().GetByEmail(gomock.Any(), email).Return(dbUser, nil)
			},
			expectedErr: nil,
		},
		{
			name:      "User Not Found",
			loginData: models.UserLogin{Email: "wrong@test.com", Password: password},
			setup: func() {
				mockRepo.EXPECT().GetByEmail(gomock.Any(), "wrong@test.com").Return(nil, repository.ErrNotFound)
			},
			expectedErr: service.ErrInvalidEmailOrPassword,
		},
		{
			name:      "Invalid Password",
			loginData: models.UserLogin{Email: email, Password: "wrong_password"},
			setup: func() {
				mockRepo.EXPECT().GetByEmail(gomock.Any(), email).Return(dbUser, nil)
			},
			expectedErr: service.ErrInvalidEmailOrPassword,
		},
		{
			name:      "Repo Internal Error",
			loginData: models.UserLogin{Email: email, Password: password},
			setup: func() {
				mockRepo.EXPECT().GetByEmail(gomock.Any(), email).Return(nil, errors.New("conn failed"))
			},
			expectedErr: errors.New("conn failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

			tt.setup()
			res, err := svc.Login(ctx, tt.loginData)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, dbUser.ID, res.ID)
			}
		})
	}
}

func TestUserService_GetOrCreateDummy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewUserService(mockRepo, 0)

	t.Run("Success", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))

		mockRepo.EXPECT().
			GetOrCreateForDummy(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, u *models.User) error {
				assert.Equal(t, models.RoleAdmin, u.Role)
				assert.Contains(t, u.Email, "dummy_")
				return nil
			})

		res, err := svc.GetOrCreateDummy(ctx, models.RoleAdmin)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Repo Error", func(t *testing.T) {
		ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
		mockRepo.EXPECT().GetOrCreateForDummy(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

		res, err := svc.GetOrCreateDummy(ctx, models.RoleUser)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func ptr[T any](v T) *T {
	return &v
}
