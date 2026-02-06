package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestMongoAccountingRepositoryCreateRecord(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success with default recorded_at", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		record := &models.AccountingRecord{
			ChatID:       -1001,
			UserID:       2001,
			Amount:       100.5,
			Currency:     models.CurrencyCNY,
			OriginalExpr: "100.5",
		}

		if err := repo.CreateRecord(context.Background(), record); err != nil {
			t.Fatalf("CreateRecord failed: %v", err)
		}
		if record.CreatedAt.IsZero() {
			t.Fatalf("expected created_at to be set")
		}
		if record.RecordedAt.IsZero() {
			t.Fatalf("expected recorded_at to be set")
		}
	})

	mt.Run("success keeps provided recorded_at", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		provided := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
		record := &models.AccountingRecord{
			ChatID:       -1002,
			UserID:       2002,
			Amount:       -30,
			Currency:     models.CurrencyUSD,
			OriginalExpr: "-30",
			RecordedAt:   provided,
		}

		if err := repo.CreateRecord(context.Background(), record); err != nil {
			t.Fatalf("CreateRecord failed: %v", err)
		}
		if !record.RecordedAt.Equal(provided) {
			t.Fatalf("recorded_at changed unexpectedly: got %v, want %v", record.RecordedAt, provided)
		}
	})

	mt.Run("insert error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    123,
			Name:    "WriteError",
			Message: "mock insert failure",
		}))

		err := repo.CreateRecord(context.Background(), &models.AccountingRecord{
			ChatID:   -1003,
			UserID:   2003,
			Amount:   1,
			Currency: models.CurrencyCNY,
		})
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to create accounting record") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoAccountingRepositoryGetRecordsByDateRange(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			accountingNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "chat_id", Value: int64(-2001)},
				{Key: "user_id", Value: int64(3001)},
				{Key: "amount", Value: 100.0},
				{Key: "currency", Value: models.CurrencyCNY},
				{Key: "recorded_at", Value: now.Add(-time.Hour)},
				{Key: "created_at", Value: now},
			},
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "chat_id", Value: int64(-2001)},
				{Key: "user_id", Value: int64(3002)},
				{Key: "amount", Value: -20.0},
				{Key: "currency", Value: models.CurrencyCNY},
				{Key: "recorded_at", Value: now},
				{Key: "created_at", Value: now},
			},
		))

		records, err := repo.GetRecordsByDateRange(
			context.Background(),
			-2001,
			now.Add(-24*time.Hour),
			now.Add(time.Hour),
			models.CurrencyCNY,
		)
		if err != nil {
			t.Fatalf("GetRecordsByDateRange failed: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("unexpected record count: got %d, want %d", len(records), 2)
		}
	})

	mt.Run("find error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.GetRecordsByDateRange(
			context.Background(),
			-2002,
			time.Now().Add(-time.Hour),
			time.Now(),
			"",
		)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to query accounting records") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("decode error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			accountingNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "chat_id", Value: "bad-type"},
				{Key: "amount", Value: 1.0},
				{Key: "currency", Value: models.CurrencyCNY},
				{Key: "recorded_at", Value: time.Now().UTC()},
				{Key: "created_at", Value: time.Now().UTC()},
			},
		))

		_, err := repo.GetRecordsByDateRange(
			context.Background(),
			-2003,
			time.Now().Add(-time.Hour),
			time.Now(),
			"",
		)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode accounting records") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoAccountingRepositoryGetRecentRecords(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			accountingNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "chat_id", Value: int64(-3001)},
				{Key: "user_id", Value: int64(4001)},
				{Key: "amount", Value: -8.0},
				{Key: "currency", Value: models.CurrencyUSD},
				{Key: "recorded_at", Value: now},
				{Key: "created_at", Value: now},
			},
		))

		records, err := repo.GetRecentRecords(context.Background(), -3001, 7)
		if err != nil {
			t.Fatalf("GetRecentRecords failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("unexpected record count: got %d, want %d", len(records), 1)
		}
	})

	mt.Run("find error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.GetRecentRecords(context.Background(), -3002, 3)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to query recent accounting records") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("decode error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			accountingNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "chat_id", Value: int64(-3003)},
				{Key: "user_id", Value: "bad-type"},
				{Key: "amount", Value: 1.0},
				{Key: "currency", Value: models.CurrencyUSD},
				{Key: "recorded_at", Value: time.Now().UTC()},
				{Key: "created_at", Value: time.Now().UTC()},
			},
		))

		_, err := repo.GetRecentRecords(context.Background(), -3003, 3)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode accounting records") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoAccountingRepositoryDeleteRecord(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("invalid object id", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}

		err := repo.DeleteRecord(context.Background(), "not-hex")
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "invalid record ID") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
		))

		id := primitive.NewObjectID().Hex()
		if err := repo.DeleteRecord(context.Background(), id); err != nil {
			t.Fatalf("DeleteRecord failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
		))

		id := primitive.NewObjectID().Hex()
		err := repo.DeleteRecord(context.Background(), id)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "record not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("delete error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    91,
			Name:    "ShutdownInProgress",
			Message: "mock delete failure",
		}))

		id := primitive.NewObjectID().Hex()
		err := repo.DeleteRecord(context.Background(), id)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to delete accounting record") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoAccountingRepositoryDeleteAllByChatID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 3},
		))

		deleted, err := repo.DeleteAllByChatID(context.Background(), -4001)
		if err != nil {
			t.Fatalf("DeleteAllByChatID failed: %v", err)
		}
		if deleted != 3 {
			t.Fatalf("unexpected deleted count: got %d, want %d", deleted, 3)
		}
	})

	mt.Run("delete many error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Name:    "MaxTimeMSExpired",
			Message: "mock delete many timeout",
		}))

		_, err := repo.DeleteAllByChatID(context.Background(), -4002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to delete all accounting records") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoAccountingRepositoryEnsureIndexes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		if err := repo.EnsureIndexes(context.Background()); err != nil {
			t.Fatalf("EnsureIndexes failed: %v", err)
		}
	})

	mt.Run("create indexes error", func(mt *mtest.T) {
		repo := &MongoAccountingRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    85,
			Name:    "IndexOptionsConflict",
			Message: "mock index error",
		}))

		err := repo.EnsureIndexes(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to create accounting indexes") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func accountingNamespace(mt *mtest.T) string {
	return mt.DB.Name() + "." + mt.Coll.Name()
}
