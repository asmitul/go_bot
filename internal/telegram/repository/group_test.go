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

func TestMongoGroupRepositoryCreateOrUpdate(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		group := &models.Group{
			TelegramID:  -1001,
			Type:        "supergroup",
			Title:       "Test Group",
			MemberCount: 10,
			BotStatus:   models.BotStatusActive,
		}

		if err := repo.CreateOrUpdate(context.Background(), group); err != nil {
			t.Fatalf("CreateOrUpdate failed: %v", err)
		}
		if group.UpdatedAt.IsZero() {
			t.Fatalf("expected updated_at to be set")
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    123,
			Name:    "WriteError",
			Message: "mock write failure",
		}))

		err := repo.CreateOrUpdate(context.Background(), &models.Group{
			TelegramID: -1002,
			Type:       "supergroup",
			Title:      "Error Group",
			BotStatus:  models.BotStatusActive,
		})
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to create or update group") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryGetByTelegramID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: int64(-2001)},
				{Key: "type", Value: "supergroup"},
				{Key: "title", Value: "Alpha"},
				{Key: "bot_status", Value: models.BotStatusActive},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		group, err := repo.GetByTelegramID(context.Background(), -2001)
		if err != nil {
			t.Fatalf("GetByTelegramID failed: %v", err)
		}
		if group.Title != "Alpha" {
			t.Fatalf("unexpected title: got %q, want %q", group.Title, "Alpha")
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
		))

		_, err := repo.GetByTelegramID(context.Background(), -2999)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "group not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("find one error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    2,
			Name:    "BadValue",
			Message: "mock find failure",
		}))

		_, err := repo.GetByTelegramID(context.Background(), -2002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to get group") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryFindByInterfaceID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("empty interface id", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}

		_, err := repo.FindByInterfaceID(context.Background(), "   ")
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "interface id is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: int64(-3001)},
				{Key: "type", Value: "supergroup"},
				{Key: "title", Value: "Interface Group"},
				{Key: "bot_status", Value: models.BotStatusActive},
				{Key: "settings", Value: bson.D{
					{Key: "interface_bindings", Value: bson.A{
						bson.D{{Key: "id", Value: "INTF-01"}, {Key: "name", Value: "Main"}},
					}},
				}},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		group, err := repo.FindByInterfaceID(context.Background(), "intf-01")
		if err != nil {
			t.Fatalf("FindByInterfaceID failed: %v", err)
		}
		if group == nil || group.TelegramID != -3001 {
			t.Fatalf("unexpected group: %+v", group)
		}
	})

	mt.Run("not found returns nil", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
		))

		group, err := repo.FindByInterfaceID(context.Background(), "NOT-EXIST")
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if group != nil {
			t.Fatalf("expected nil group for not found, got: %+v", group)
		}
	})

	mt.Run("find one error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.FindByInterfaceID(context.Background(), "INTF-02")
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to find group by interface id") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryUpdateBotStatus(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		if err := repo.UpdateBotStatus(context.Background(), -4001, models.BotStatusLeft); err != nil {
			t.Fatalf("UpdateBotStatus failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
			bson.E{Key: "nModified", Value: 0},
		))

		err := repo.UpdateBotStatus(context.Background(), -4002, models.BotStatusKicked)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "group not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    112,
			Name:    "WriteConflict",
			Message: "mock update conflict",
		}))

		err := repo.UpdateBotStatus(context.Background(), -4003, models.BotStatusLeft)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to update bot status") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryDeleteGroup(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
		))

		if err := repo.DeleteGroup(context.Background(), -5001); err != nil {
			t.Fatalf("DeleteGroup failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
		))

		err := repo.DeleteGroup(context.Background(), -5002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "group not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("delete error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    91,
			Name:    "ShutdownInProgress",
			Message: "mock delete failure",
		}))

		err := repo.DeleteGroup(context.Background(), -5003)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to delete group") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryListGroups(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("list all success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: int64(-6001)},
				{Key: "type", Value: "supergroup"},
				{Key: "title", Value: "All 1"},
				{Key: "bot_status", Value: models.BotStatusActive},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
			bson.D{
				{Key: "telegram_id", Value: int64(-6002)},
				{Key: "type", Value: "group"},
				{Key: "title", Value: "All 2"},
				{Key: "bot_status", Value: models.BotStatusLeft},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		groups, err := repo.ListAllGroups(context.Background())
		if err != nil {
			t.Fatalf("ListAllGroups failed: %v", err)
		}
		if len(groups) != 2 {
			t.Fatalf("unexpected group count: got %d, want %d", len(groups), 2)
		}
	})

	mt.Run("list all find error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.ListAllGroups(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to list groups") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("list all decode error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: "not-int64"},
				{Key: "title", Value: "broken"},
			},
		))

		_, err := repo.ListAllGroups(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode groups") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("list active success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: int64(-7001)},
				{Key: "type", Value: "supergroup"},
				{Key: "title", Value: "Active 1"},
				{Key: "bot_status", Value: models.BotStatusActive},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		groups, err := repo.ListActiveGroups(context.Background())
		if err != nil {
			t.Fatalf("ListActiveGroups failed: %v", err)
		}
		if len(groups) != 1 {
			t.Fatalf("unexpected active group count: got %d, want %d", len(groups), 1)
		}
		if groups[0].BotStatus != models.BotStatusActive {
			t.Fatalf("unexpected bot status: %q", groups[0].BotStatus)
		}
	})

	mt.Run("list active find error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.ListActiveGroups(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to list active groups") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("list active decode error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			groupNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_id", Value: "not-int64"},
				{Key: "bot_status", Value: models.BotStatusActive},
			},
		))

		_, err := repo.ListActiveGroups(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode groups") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryUpdateSettings(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	settings := models.GroupSettings{
		CalculatorEnabled: true,
		CryptoEnabled:     true,
		ForwardEnabled:    true,
	}

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		if err := repo.UpdateSettings(
			context.Background(),
			-8001,
			settings,
			models.GroupTierMerchant,
		); err != nil {
			t.Fatalf("UpdateSettings failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
			bson.E{Key: "nModified", Value: 0},
		))

		err := repo.UpdateSettings(context.Background(), -8002, settings, models.GroupTierBasic)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "group not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    112,
			Name:    "WriteConflict",
			Message: "mock write conflict",
		}))

		err := repo.UpdateSettings(context.Background(), -8003, settings, models.GroupTierUpstream)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to update settings") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryUpdateStats(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	stats := models.GroupStats{
		TotalMessages: 42,
		LastMessageAt: time.Now().UTC(),
	}

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		if err := repo.UpdateStats(context.Background(), -9001, stats); err != nil {
			t.Fatalf("UpdateStats failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
			bson.E{Key: "nModified", Value: 0},
		))

		err := repo.UpdateStats(context.Background(), -9002, stats)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "group not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    101,
			Name:    "NotWritablePrimary",
			Message: "mock primary stepdown",
		}))

		err := repo.UpdateStats(context.Background(), -9003, stats)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to update stats") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoGroupRepositoryEnsureIndexes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		if err := repo.EnsureIndexes(context.Background(), 0); err != nil {
			t.Fatalf("EnsureIndexes failed: %v", err)
		}
	})

	mt.Run("create indexes error", func(mt *mtest.T) {
		repo := &MongoGroupRepository{collection: mt.Coll}
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

func groupNamespace(mt *mtest.T) string {
	return mt.DB.Name() + "." + mt.Coll.Name()
}
