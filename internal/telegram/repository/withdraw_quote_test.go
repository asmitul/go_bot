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

func TestMongoWithdrawQuoteRepositoryUpsert(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("upsert by withdraw no success", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := repo.Upsert(context.Background(), &models.WithdrawQuoteRecord{
			MerchantID: 2023001,
			WithdrawNo: "W-1",
			Amount:     694,
			Rate:       6.94,
			USDTAmount: 100,
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	})

	mt.Run("upsert by order no success", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := repo.Upsert(context.Background(), &models.WithdrawQuoteRecord{
			MerchantID: 2023001,
			OrderNo:    "O-1",
			Amount:     694,
			Rate:       6.94,
			USDTAmount: 100,
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	})

	mt.Run("insert without keys success", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := repo.Upsert(context.Background(), &models.WithdrawQuoteRecord{
			MerchantID: 2023001,
			Amount:     300,
			Rate:       6.5,
			USDTAmount: 46.15,
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	})

	mt.Run("nil record", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		err := repo.Upsert(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "record is nil") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("missing merchant id", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		err := repo.Upsert(context.Background(), &models.WithdrawQuoteRecord{})
		if err == nil || !strings.Contains(err.Error(), "merchant id is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("upsert command error", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    91,
			Name:    "ShutdownInProgress",
			Message: "mock upsert failure",
		}))

		err := repo.Upsert(context.Background(), &models.WithdrawQuoteRecord{
			MerchantID: 2023001,
			WithdrawNo: "W-ERR",
			Amount:     1,
			Rate:       1,
			USDTAmount: 1,
		})
		if err == nil || !strings.Contains(err.Error(), "failed to upsert withdraw quote record") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoWithdrawQuoteRepositoryListByMerchantAndDateRange(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			withdrawQuoteNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "merchant_id", Value: int64(2023001)},
				{Key: "withdraw_no", Value: "W-1"},
				{Key: "amount", Value: 694.0},
				{Key: "rate", Value: 6.94},
				{Key: "usdt_amount", Value: 100.0},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		records, err := repo.ListByMerchantAndDateRange(
			context.Background(),
			2023001,
			now.Add(-time.Hour),
			now.Add(time.Hour),
		)
		if err != nil {
			t.Fatalf("ListByMerchantAndDateRange failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("unexpected record count: got %d, want %d", len(records), 1)
		}
	})

	mt.Run("find error", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.ListByMerchantAndDateRange(context.Background(), 2023001, time.Now().Add(-time.Hour), time.Now())
		if err == nil || !strings.Contains(err.Error(), "failed to query withdraw quote records") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("decode error", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			withdrawQuoteNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "merchant_id", Value: "bad-type"},
				{Key: "created_at", Value: time.Now().UTC()},
				{Key: "updated_at", Value: time.Now().UTC()},
			},
		))

		_, err := repo.ListByMerchantAndDateRange(context.Background(), 2023001, time.Now().Add(-time.Hour), time.Now())
		if err == nil || !strings.Contains(err.Error(), "failed to decode withdraw quote records") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("missing merchant id", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		_, err := repo.ListByMerchantAndDateRange(context.Background(), 0, time.Now().Add(-time.Hour), time.Now())
		if err == nil || !strings.Contains(err.Error(), "merchant id is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoWithdrawQuoteRepositoryEnsureIndexes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		if err := repo.EnsureIndexes(context.Background()); err != nil {
			t.Fatalf("EnsureIndexes failed: %v", err)
		}
	})

	mt.Run("create index error", func(mt *mtest.T) {
		repo := &MongoWithdrawQuoteRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    85,
			Name:    "IndexOptionsConflict",
			Message: "mock index failure",
		}))

		err := repo.EnsureIndexes(context.Background())
		if err == nil || !strings.Contains(err.Error(), "failed to create withdraw quote indexes") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func withdrawQuoteNamespace(mt *mtest.T) string {
	return mt.DB.Name() + "." + mt.Coll.Name()
}
