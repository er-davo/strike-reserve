//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"booking-service/internal/models"
	"booking-service/internal/repository"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_FullCycle(t *testing.T) {
	ctx := context.Background()
	trxMngr := manager.Must(trmpgx.NewDefaultFactory(db))
	repo := repository.NewUserRepository(db, trmpgx.DefaultCtxGetter)

	regID := uuid.New()
	regEmail := "register@test.com"
	regHash := "$2a$12$V9..."

	err := trxMngr.Do(ctx, func(ctx context.Context) error {
		t.Run("Standard Registration Success", func(t *testing.T) {
			newUser := &models.User{
				ID:           regID,
				Email:        regEmail,
				PasswordHash: &regHash,
				Role:         models.RoleUser,
			}
			err := repo.Create(ctx, newUser)
			require.NoError(t, err)

			found, err := repo.GetByEmail(ctx, regEmail)
			require.NoError(t, err)
			require.NotNil(t, found.PasswordHash)
			assert.Equal(t, regHash, *found.PasswordHash)
		})

		t.Run("Registration Duplicate Email Error", func(t *testing.T) {
			duplicateUser := &models.User{
				ID:    uuid.New(),
				Email: regEmail,
				Role:  models.RoleUser,
			}
			err := repo.Create(ctx, duplicateUser)
			assert.Error(t, err, "DB must return error on duplicate email")
		})

		return fmt.Errorf("rollback_all")
	})

	err = trxMngr.Do(ctx, func(ctx context.Context) error {
		t.Run("Dummy Login for existing user", func(t *testing.T) {
			dummyUser := &models.User{
				ID:    regID,
				Email: regEmail,
				Role:  models.RoleUser,
			}
			err := repo.GetOrCreateForDummy(ctx, dummyUser)
			require.NoError(t, err)
		})

		t.Run("GetByID Validation", func(t *testing.T) {
			found, err := repo.GetByID(ctx, regID)
			require.NoError(t, err)
			assert.Equal(t, regEmail, found.Email)
		})

		t.Run("Create User without password (Dummy style)", func(t *testing.T) {
			noPassID := uuid.New()
			noPassEmail := "nopass@test.com"

			user := &models.User{ID: noPassID, Email: noPassEmail, Role: models.RoleAdmin}
			err := repo.GetOrCreateForDummy(ctx, user)
			require.NoError(t, err)

			found, err := repo.GetByEmail(ctx, noPassEmail)
			require.NoError(t, err)
			assert.Nil(t, found.PasswordHash, "Dummy user's password hash must be NULL")
		})

		return fmt.Errorf("rollback_all")
	})

	assert.Contains(t, err.Error(), "rollback_all")
}
