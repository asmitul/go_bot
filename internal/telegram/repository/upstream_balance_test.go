package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestMongoUpstreamBalanceRepositoryGet(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)

		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{
				Key: "value",
				Value: bson.D{
					{Key: "group_id", Value: int64(-1001)},
					{Key: "balance", Value: 88.8},
					{Key: "min_balance", Value: 10.0},
					{Key: "alert_limit_per_hour", Value: 3},
					{Key: "created_at", Value: now},
					{Key: "updated_at", Value: now},
				},
			},
		))

		balance, err := repo.Get(context.Background(), -1001)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if balance.GroupID != -1001 {
			t.Fatalf("unexpected group id: got %d, want %d", balance.GroupID, -1001)
		}
		if balance.Balance != 88.8 {
			t.Fatalf("unexpected balance: got %.2f, want %.2f", balance.Balance, 88.8)
		}
	})

	mt.Run("find one and update error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock findAndModify error",
		}))

		_, err := repo.Get(context.Background(), -1002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to get upstream balance") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUpstreamBalanceRepositoryAdjustWithoutTransaction(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success without operation id", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)

		mt.AddMockResponses(
			mtest.CreateSuccessResponse(
				bson.E{
					Key: "value",
					Value: bson.D{
						{Key: "group_id", Value: int64(-2001)},
						{Key: "balance", Value: 120.0},
						{Key: "min_balance", Value: 30.0},
						{Key: "alert_limit_per_hour", Value: 3},
						{Key: "created_at", Value: now},
						{Key: "updated_at", Value: now},
					},
				},
			),
			mtest.CreateSuccessResponse(),
		)

		balance, err := repo.adjustWithoutTransaction(
			context.Background(),
			-2001,
			20,
			9001,
			"manual credit",
			models.BalanceOpCredit,
			"",
			nil,
		)
		if err != nil {
			t.Fatalf("adjustWithoutTransaction failed: %v", err)
		}
		if balance.Balance != 120.0 {
			t.Fatalf("unexpected balance: got %.2f, want %.2f", balance.Balance, 120.0)
		}
	})

	mt.Run("idempotent returns current balance when log exists", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)

		mt.AddMockResponses(
			mtest.CreateCursorResponse(
				0,
				upstreamLogNamespace(mt),
				mtest.FirstBatch,
				bson.D{
					{Key: "group_id", Value: int64(-2002)},
					{Key: "operation_id", Value: "op-1"},
					{Key: "type", Value: string(models.BalanceOpCredit)},
					{Key: "created_at", Value: now},
				},
			),
			mtest.CreateSuccessResponse(
				bson.E{
					Key: "value",
					Value: bson.D{
						{Key: "group_id", Value: int64(-2002)},
						{Key: "balance", Value: 66.0},
						{Key: "min_balance", Value: 10.0},
						{Key: "alert_limit_per_hour", Value: 3},
						{Key: "created_at", Value: now},
						{Key: "updated_at", Value: now},
					},
				},
			),
		)

		balance, err := repo.adjustWithoutTransaction(
			context.Background(),
			-2002,
			100,
			9002,
			"idempotent check",
			models.BalanceOpCredit,
			"op-1",
			nil,
		)
		if err != nil {
			t.Fatalf("adjustWithoutTransaction failed: %v", err)
		}
		if balance.Balance != 66.0 {
			t.Fatalf("unexpected balance: got %.2f, want %.2f", balance.Balance, 66.0)
		}
	})

	mt.Run("update balance error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    112,
			Name:    "WriteConflict",
			Message: "mock update conflict",
		}))

		_, err := repo.adjustWithoutTransaction(
			context.Background(),
			-2003,
			-10,
			9003,
			"manual debit",
			models.BalanceOpDebit,
			"",
			nil,
		)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "update balance failed (non-txn)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("insert log error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(
				bson.E{
					Key: "value",
					Value: bson.D{
						{Key: "group_id", Value: int64(-2004)},
						{Key: "balance", Value: 50.0},
						{Key: "min_balance", Value: 0.0},
						{Key: "alert_limit_per_hour", Value: 3},
						{Key: "created_at", Value: now},
						{Key: "updated_at", Value: now},
					},
				},
			),
			mtest.CreateCommandErrorResponse(mtest.CommandError{
				Code:    91,
				Name:    "ShutdownInProgress",
				Message: "mock insert failure",
			}),
		)

		_, err := repo.adjustWithoutTransaction(
			context.Background(),
			-2004,
			10,
			9004,
			"insert log fail",
			models.BalanceOpCredit,
			"",
			nil,
		)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "insert balance log failed (non-txn)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUpstreamBalanceRepositoryUpdateSettingsWithoutTransaction(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(
				bson.E{
					Key: "value",
					Value: bson.D{
						{Key: "group_id", Value: int64(-3001)},
						{Key: "balance", Value: 80.0},
						{Key: "min_balance", Value: 25.0},
						{Key: "alert_limit_per_hour", Value: 3},
						{Key: "created_at", Value: now},
						{Key: "updated_at", Value: now},
					},
				},
			),
			mtest.CreateSuccessResponse(),
		)

		balance, err := repo.updateSettingsWithoutTransaction(
			context.Background(),
			-3001,
			bson.M{"min_balance": 25.0},
			9001,
			models.BalanceOpSetMinBalance,
			"set min",
		)
		if err != nil {
			t.Fatalf("updateSettingsWithoutTransaction failed: %v", err)
		}
		if balance.MinBalance != 25.0 {
			t.Fatalf("unexpected min balance: got %.2f, want %.2f", balance.MinBalance, 25.0)
		}
	})

	mt.Run("update settings error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    112,
			Name:    "WriteConflict",
			Message: "mock update settings conflict",
		}))

		_, err := repo.updateSettingsWithoutTransaction(
			context.Background(),
			-3002,
			bson.M{"alert_limit_per_hour": 5},
			9002,
			models.BalanceOpAlertLimit,
			"set limit",
		)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "update balance settings failed (non-txn)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("insert log error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(
				bson.E{
					Key: "value",
					Value: bson.D{
						{Key: "group_id", Value: int64(-3003)},
						{Key: "balance", Value: 90.0},
						{Key: "min_balance", Value: 20.0},
						{Key: "alert_limit_per_hour", Value: 6},
						{Key: "created_at", Value: now},
						{Key: "updated_at", Value: now},
					},
				},
			),
			mtest.CreateCommandErrorResponse(mtest.CommandError{
				Code:    91,
				Name:    "ShutdownInProgress",
				Message: "mock insert log failure",
			}),
		)

		_, err := repo.updateSettingsWithoutTransaction(
			context.Background(),
			-3003,
			bson.M{"alert_limit_per_hour": 6},
			9003,
			models.BalanceOpAlertLimit,
			"set limit",
		)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "insert balance log failed (non-txn)") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUpstreamBalanceRepositoryListAll(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			upstreamBalanceNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "group_id", Value: int64(-4001)},
				{Key: "balance", Value: 100.0},
				{Key: "min_balance", Value: 30.0},
				{Key: "alert_limit_per_hour", Value: 3},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
			bson.D{
				{Key: "group_id", Value: int64(-4002)},
				{Key: "balance", Value: 50.0},
				{Key: "min_balance", Value: 10.0},
				{Key: "alert_limit_per_hour", Value: 4},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		balances, err := repo.ListAll(context.Background())
		if err != nil {
			t.Fatalf("ListAll failed: %v", err)
		}
		if len(balances) != 2 {
			t.Fatalf("unexpected balance count: got %d, want %d", len(balances), 2)
		}
	})

	mt.Run("find error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.ListAll(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "list balances failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("decode error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			upstreamBalanceNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "group_id", Value: "bad-type"},
				{Key: "balance", Value: 1.0},
				{Key: "min_balance", Value: 0.0},
				{Key: "created_at", Value: time.Now().UTC()},
				{Key: "updated_at", Value: time.Now().UTC()},
			},
		))

		_, err := repo.ListAll(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "decode balances failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUpstreamBalanceRepositoryEnsureIndexes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(),
			mtest.CreateSuccessResponse(),
		)

		if err := repo.EnsureIndexes(context.Background()); err != nil {
			t.Fatalf("EnsureIndexes failed: %v", err)
		}
	})

	mt.Run("balance indexes error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    85,
			Name:    "IndexOptionsConflict",
			Message: "mock balance index error",
		}))

		err := repo.EnsureIndexes(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "create balance indexes") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("log indexes error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(),
			mtest.CreateCommandErrorResponse(mtest.CommandError{
				Code:    85,
				Name:    "IndexOptionsConflict",
				Message: "mock log index error",
			}),
		)

		err := repo.EnsureIndexes(context.Background())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "create balance log indexes") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoUpstreamBalanceRepositoryFindLogByOperation(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("empty operation id", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)

		log, err := repo.findLogByOperation(context.Background(), -5001, "")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if log != nil {
			t.Fatalf("expected nil log, got %+v", log)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			upstreamLogNamespace(mt),
			mtest.FirstBatch,
		))

		log, err := repo.findLogByOperation(context.Background(), -5002, "op-2")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if log != nil {
			t.Fatalf("expected nil log, got %+v", log)
		}
	})

	mt.Run("found", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			upstreamLogNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "group_id", Value: int64(-5003)},
				{Key: "operator_id", Value: int64(9001)},
				{Key: "delta", Value: 10.0},
				{Key: "balance", Value: 100.0},
				{Key: "type", Value: string(models.BalanceOpCredit)},
				{Key: "operation_id", Value: "op-3"},
				{Key: "created_at", Value: now},
			},
		))

		log, err := repo.findLogByOperation(context.Background(), -5003, "op-3")
		if err != nil {
			t.Fatalf("findLogByOperation failed: %v", err)
		}
		if log == nil || log.OperationID != "op-3" {
			t.Fatalf("unexpected log: %+v", log)
		}
	})

	mt.Run("find error", func(mt *mtest.T) {
		repo := newUpstreamRepoForTest(mt)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find log error",
		}))

		_, err := repo.findLogByOperation(context.Background(), -5004, "op-4")
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
	})
}

func TestUpstreamBalanceHelpers(t *testing.T) {
	t.Run("mergeBson", func(t *testing.T) {
		result := mergeBson(bson.M{"a": 1, "b": 2}, bson.M{"b": 3, "c": 4})
		expected := bson.M{"a": 1, "b": 3, "c": 4}
		if !reflect.DeepEqual(result, expected) {
			t.Fatalf("unexpected merge result: got %#v, want %#v", result, expected)
		}
	})

	t.Run("filterDefaults", func(t *testing.T) {
		result := filterDefaults(
			bson.M{"min_balance": 0.0, "alert_limit_per_hour": 3, "created_at": "x"},
			bson.M{"min_balance": 5.0},
		)
		expected := bson.M{"alert_limit_per_hour": 3, "created_at": "x"}
		if !reflect.DeepEqual(result, expected) {
			t.Fatalf("unexpected filtered defaults: got %#v, want %#v", result, expected)
		}
	})

	t.Run("balanceFilter", func(t *testing.T) {
		filter := balanceFilter(-6001)
		want := bson.M{
			"$or": []bson.M{
				{"group_id": int64(-6001)},
				{"chat_id": int64(-6001)},
			},
		}
		if !reflect.DeepEqual(filter, want) {
			t.Fatalf("unexpected filter: got %#v, want %#v", filter, want)
		}
	})

	t.Run("isTransactionNotSupported true by code", func(t *testing.T) {
		err := mongo.CommandError{
			Code:    20,
			Name:    "IllegalOperation",
			Message: "transactions not allowed",
		}
		if !isTransactionNotSupported(err) {
			t.Fatalf("expected true for IllegalOperation")
		}
	})

	t.Run("isTransactionNotSupported true by message", func(t *testing.T) {
		err := errors.New("Transaction numbers are only allowed on a replica set member or mongos")
		if !isTransactionNotSupported(err) {
			t.Fatalf("expected true for transaction-not-supported message")
		}
	})

	t.Run("isTransactionNotSupported false", func(t *testing.T) {
		err := mongo.CommandError{
			Code:    2,
			Name:    "BadValue",
			Message: "other error",
		}
		if isTransactionNotSupported(err) {
			t.Fatalf("expected false for unrelated command error")
		}
	})
}

func newUpstreamRepoForTest(mt *mtest.T) *MongoUpstreamBalanceRepository {
	return &MongoUpstreamBalanceRepository{
		balanceColl: mt.DB.Collection("upstream_balances"),
		logColl:     mt.DB.Collection("upstream_balance_logs"),
	}
}

func upstreamBalanceNamespace(mt *mtest.T) string {
	return mt.DB.Name() + ".upstream_balances"
}

func upstreamLogNamespace(mt *mtest.T) string {
	return mt.DB.Name() + ".upstream_balance_logs"
}
