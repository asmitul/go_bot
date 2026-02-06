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

func TestMongoMessageRepositoryCreateMessage(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		msg := &models.Message{
			TelegramMessageID: 1001,
			ChatID:            -2001,
			UserID:            3001,
			MessageType:       models.MessageTypeText,
			Text:              "hello",
			SentAt:            time.Now().UTC(),
		}

		if err := repo.CreateMessage(context.Background(), msg); err != nil {
			t.Fatalf("CreateMessage failed: %v", err)
		}
		if msg.CreatedAt.IsZero() || msg.UpdatedAt.IsZero() {
			t.Fatalf("expected created_at and updated_at to be set")
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    123,
			Name:    "WriteError",
			Message: "mock write failure",
		}))

		err := repo.CreateMessage(context.Background(), &models.Message{
			TelegramMessageID: 1002,
			ChatID:            -2002,
			MessageType:       models.MessageTypeText,
			SentAt:            time.Now().UTC(),
		})
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to create message") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoMessageRepositoryGetByTelegramID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			messageNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_message_id", Value: int64(5001)},
				{Key: "chat_id", Value: int64(-6001)},
				{Key: "user_id", Value: int64(7001)},
				{Key: "message_type", Value: models.MessageTypeText},
				{Key: "text", Value: "saved"},
				{Key: "sent_at", Value: now},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		msg, err := repo.GetByTelegramID(context.Background(), 5001, -6001)
		if err != nil {
			t.Fatalf("GetByTelegramID failed: %v", err)
		}
		if msg.Text != "saved" {
			t.Fatalf("unexpected text: got %q, want %q", msg.Text, "saved")
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			messageNamespace(mt),
			mtest.FirstBatch,
		))

		_, err := repo.GetByTelegramID(context.Background(), 9999, -1)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "message not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("find one error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    2,
			Name:    "BadValue",
			Message: "mock find failure",
		}))

		_, err := repo.GetByTelegramID(context.Background(), 5002, -6002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to get message") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoMessageRepositoryUpdateMessageEdit(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		))

		err := repo.UpdateMessageEdit(
			context.Background(),
			10001,
			-20001,
			"edited",
			time.Now().UTC(),
		)
		if err != nil {
			t.Fatalf("UpdateMessageEdit failed: %v", err)
		}
	})

	mt.Run("not found", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 0},
			bson.E{Key: "nModified", Value: 0},
		))

		err := repo.UpdateMessageEdit(context.Background(), 10002, -20002, "edited", time.Now().UTC())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "message not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("update error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    112,
			Name:    "WriteConflict",
			Message: "mock update conflict",
		}))

		err := repo.UpdateMessageEdit(context.Background(), 10003, -20003, "edited", time.Now().UTC())
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to update message edit") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoMessageRepositoryListMessagesByChat(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		now := time.Now().UTC().Truncate(time.Second)
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			messageNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_message_id", Value: int64(2)},
				{Key: "chat_id", Value: int64(-777)},
				{Key: "message_type", Value: models.MessageTypeText},
				{Key: "text", Value: "newest"},
				{Key: "sent_at", Value: now},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
			bson.D{
				{Key: "telegram_message_id", Value: int64(1)},
				{Key: "chat_id", Value: int64(-777)},
				{Key: "message_type", Value: models.MessageTypeText},
				{Key: "text", Value: "older"},
				{Key: "sent_at", Value: now.Add(-time.Minute)},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		))

		messages, err := repo.ListMessagesByChat(context.Background(), -777, 10, 0)
		if err != nil {
			t.Fatalf("ListMessagesByChat failed: %v", err)
		}
		if len(messages) != 2 {
			t.Fatalf("unexpected count: got %d, want %d", len(messages), 2)
		}
		if messages[0].Text != "newest" {
			t.Fatalf("unexpected order, first text: %q", messages[0].Text)
		}
	})

	mt.Run("find error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    13,
			Name:    "Unauthorized",
			Message: "mock find error",
		}))

		_, err := repo.ListMessagesByChat(context.Background(), -1, 5, 0)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to list messages") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("decode error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			messageNamespace(mt),
			mtest.FirstBatch,
			bson.D{
				{Key: "telegram_message_id", Value: "not-int64"},
				{Key: "chat_id", Value: int64(-999)},
				{Key: "message_type", Value: models.MessageTypeText},
			},
		))

		_, err := repo.ListMessagesByChat(context.Background(), -999, 5, 0)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode messages") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoMessageRepositoryCountMessagesByType(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			messageNamespace(mt),
			mtest.FirstBatch,
			bson.D{{Key: "_id", Value: models.MessageTypeText}, {Key: "count", Value: int64(3)}},
			bson.D{{Key: "_id", Value: models.MessageTypePhoto}, {Key: "count", Value: int64(1)}},
		))

		counts, err := repo.CountMessagesByType(context.Background(), -10001)
		if err != nil {
			t.Fatalf("CountMessagesByType failed: %v", err)
		}
		if counts[models.MessageTypeText] != 3 {
			t.Fatalf("unexpected text count: got %d, want %d", counts[models.MessageTypeText], 3)
		}
		if counts[models.MessageTypePhoto] != 1 {
			t.Fatalf("unexpected photo count: got %d, want %d", counts[models.MessageTypePhoto], 1)
		}
	})

	mt.Run("aggregate error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    9,
			Name:    "FailedToParse",
			Message: "mock aggregate failure",
		}))

		_, err := repo.CountMessagesByType(context.Background(), -10002)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to count messages by type") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	mt.Run("decode error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCursorResponse(
			0,
			messageNamespace(mt),
			mtest.FirstBatch,
			bson.D{{Key: "_id", Value: 1234}, {Key: "count", Value: int64(1)}},
		))

		_, err := repo.CountMessagesByType(context.Background(), -10003)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode count result") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestMongoMessageRepositoryEnsureIndexes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		if err := repo.EnsureIndexes(context.Background(), 3600); err != nil {
			t.Fatalf("EnsureIndexes failed: %v", err)
		}
	})

	mt.Run("create indexes error", func(mt *mtest.T) {
		repo := &MongoMessageRepository{collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    85,
			Name:    "IndexOptionsConflict",
			Message: "mock index error",
		}))

		err := repo.EnsureIndexes(context.Background(), 3600)
		if err == nil {
			t.Fatalf("expected error but got nil")
		}
		if !strings.Contains(err.Error(), "failed to create indexes") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func messageNamespace(mt *mtest.T) string {
	return mt.DB.Name() + "." + mt.Coll.Name()
}
