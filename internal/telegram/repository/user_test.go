package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestMongoUserRepositoryCreateOrUpdate(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		user := &models.User{
			TelegramID:   1001,
			Username:     "tester",
			FirstName:    "Test",
			LastName:     "User",
			LanguageCode: "zh-CN",
			IsPremium:    true,
			LastActiveAt: time.Now().UTC().Add(-time.Minute),
		}

		if err := repo.CreateOrUpdate(context.Background(), user); err != nil {
			t.Fatalf("CreateOrUpdate failed: %v", err)
		}
		if user.UpdatedAt.IsZero() {
			t.Fatalf("expected updated_at to be set")
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    123,
			Name:    "WriteError",
			Message: "mock write failure",
		}))

		err := repo.CreateOrUpdate(context.Background(), &models.User{
			TelegramID: 1002,
			FirstName:  "Error",
		})
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to create or update user") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUserRepositoryGetByTelegramID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			userNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: int64(2001)},
				{Key: "username", Value: "admin_user"},
				{Key: "first_name", Value: "Admin"},
				{Key: "role", Value: models.RoleAdmin},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
				{Key: "last_active_at", Value: now},
			},
		))

		user, err := repo.GetByTelegramID(context.Background(), 2001)
		if err != nil {
			t.Fatalf("GetByTelegramID failed: %v", err)
		}
		if user.Username != "admin_user" {
			t.Fatalf("unexpected username: got %q, want %q", user.Username, "admin_user")
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			userNamespace(mt),
			mtest.FirstBatch,
		))

		_, err := repo.GetByTelegramID(context.Background(), 9999)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "user not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("find one error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    2,
			Name:    "BadValue",
			Message: "mock find failure",
		}))

		_, err := repo.GetByTelegramID(context.Background(), 3001)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to get user") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUserRepositoryUpdateLastActive(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		if err := repo.UpdateLastActive(context.Background(), 4001); err != nil {
			t.Fatalf("UpdateLastActive failed: %v", err)
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    89,
			Name:    "NetworkTimeout",
			Message: "mock timeout",
		}))

		err := repo.UpdateLastActive(context.Background(), 4002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to update last active") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUserRepositoryGrantAdmin(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		if err := repo.GrantAdmin(context.Background(), 5001, 9001); err != nil {
			t.Fatalf("GrantAdmin failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
			bson.E{Key: "nModified", Value: 0},
		))

		err := repo.GrantAdmin(context.Background(), 5002, 9002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "user not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    112,
			Name:    "WriteConflict",
			Message: "mock write conflict",
		}))

		err := repo.GrantAdmin(context.Background(), 5003, 9003)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to grant admin") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUserRepositoryRevokeAdmin(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		if err := repo.RevokeAdmin(context.Background(), 6001); err != nil {
			t.Fatalf("RevokeAdmin failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
			bson.E{Key: "nModified", Value: 0},
		))

		err := repo.RevokeAdmin(context.Background(), 6002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "user not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    101,
			Name:    "NotWritablePrimary",
			Message: "mock primary stepdown",
		}))

		err := repo.RevokeAdmin(context.Background(), 6003)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to revoke admin") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUserRepositoryListAdmins(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			userNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: int64(7001)},
				{Key: "username", Value: "owner"},
				{Key: "role", Value: models.RoleOwner},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
				{Key: "last_active_at", Value: now},
			},
			bson.D{
				{Key: "telegram_id", Value: int64(7002)},
				{Key: "username", Value: "admin"},
				{Key: "role", Value: models.RoleAdmin},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
				{Key: "last_active_at", Value: now},
			},
		))

		admins, err := repo.ListAdmins(context.Background())
		if err != nil {
			t.Fatalf("ListAdmins failed: %v", err)
		}
		if len(admins) != 2 {
			t.Fatalf("unexpected admin count: got %d, want %d", len(admins), 2)
		}
		if admins[0].Role != models.RoleOwner {
			t.Fatalf("unexpected first role: %q", admins[0].Role)
		}
	})

	mt.Run("find error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.ListAdmins(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to list admins") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("decode error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			userNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: "not-int64"},
				{Key: "role", Value: models.RoleAdmin},
			},
		))

		_, err := repo.ListAdmins(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode admins") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUserRepositoryGetUserInfo(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			userNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: int64(8001)},
				{Key: "username", Value: "info_user"},
				{Key: "first_name", Value: "Info"},
				{Key: "role", Value: models.RoleUser},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
				{Key: "last_active_at", Value: now},
			},
		))

		user, err := repo.GetUserInfo(context.Background(), 8001)
		if err != nil {
			t.Fatalf("GetUserInfo failed: %v", err)
		}
		if user.Username != "info_user" {
			t.Fatalf("unexpected username: %q", user.Username)
		}
	})
}

func TestMongoUserRepositoryEnsureIndexes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		if err := repo.EnsureIndexes(context.Background(), 0); err != nil {
			t.Fatalf("EnsureIndexes failed: %v", err)
		}
	})

	mt.Run("create indexes error", func(mt *mtest.T) {
		repo := &MongoUserRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    85,
			Name:    "IndexOptionsConflict",
			Message: "mock index error",
		}))

		err := repo.EnsureIndexes(context.Background(), 0)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to create indexes") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func userNamespace(mt *mtest.T) string {
	return mt.DB.Name() + "." + mt.Coll.Name()
}
