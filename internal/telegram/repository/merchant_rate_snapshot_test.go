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

func TestMongoMerchantRateSnapshotRepositoryGetByChatID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			merchantRateSnapshotNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "chat_id", Value: int64(-1001)},
				{Key: "merchant_id", Value: int64(2024164)},
				{Key: "rates", Value: bson.A{bson.D{
					{Key: "channel_code", Value: "wxhf"},
					{Key: "channel_name", Value: "微信话费"},
					{Key: "rate", Value: "7"},
				}}},
				{Key: "last_checked_at", Value: now},
			},
		))

		snapshot, err := repo.GetByChatID(context.Background(), -1001)
		if err != nil {
			t.Fatalf("GetByChatID failed: %v", err)
		}
		if snapshot == nil || snapshot.MerchantID != 2024164 || len(snapshot.Rates) != 1 {
			t.Fatalf("unexpected snapshot: %#v", snapshot)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			merchantRateSnapshotNamespace(mt),
			mtest.FirstBatch,
		))
		snapshot, err := repo.GetByChatID(context.Background(), -1002)
		if err != nil || snapshot != nil {
			t.Fatalf("expected nil snapshot, got %#v, err=%v", snapshot, err)
		}
	})

	mt.Run("missing chat id", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		_, err := repo.GetByChatID(context.Background(), 0)
		if err == nil || !strings.Contains(err.Error(), "chat id is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoMerchantRateSnapshotRepositoryUpsert(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())
		err := repo.Upsert(context.Background(), &models.MerchantRateSnapshot{
			ChatID:     -1001,
			MerchantID: 2024164,
			Rates: []models.MerchantChannelRate{
				{ChannelCode: "wxhf", Rate: "7"},
			},
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	})

	mt.Run("validation", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		for _, snapshot := range []*models.MerchantRateSnapshot{
			nil,
			{},
			{ChatID: -1001},
		} {
			if err := repo.Upsert(context.Background(), snapshot); err == nil {
				t.Fatalf("expected validation error for %#v", snapshot)
			}
		}
	})

	mt.Run("command error", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code: 91, Name: "ShutdownInProgress", Message: "mock failure",
		}))
		err := repo.Upsert(context.Background(), &models.MerchantRateSnapshot{
			ChatID: -1001, MerchantID: 2024164,
		})
		if err == nil || !strings.Contains(err.Error(), "failed to upsert merchant rate snapshot") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoMerchantRateSnapshotRepositoryEnsureIndexes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())
		if err := repo.EnsureIndexes(context.Background()); err != nil {
			t.Fatalf("EnsureIndexes failed: %v", err)
		}
	})

	mt.Run("error", func(mt *mtest.T) {
		repo := &MongoMerchantRateSnapshotRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code: 85, Name: "IndexOptionsConflict", Message: "mock failure",
		}))
		err := repo.EnsureIndexes(context.Background())
		if err == nil || !strings.Contains(err.Error(), "failed to create merchant rate snapshot index") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func merchantRateSnapshotNamespace(mt *mtest.T) string {
	return mt.DB.Name() + "." + mt.Coll.Name()
}
