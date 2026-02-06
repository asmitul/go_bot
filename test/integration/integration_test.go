//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	mongoclient "go_bot/internal/mongo"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"

	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

func TestMessageRepositoryIntegrationFlow(t *testing.T) {
	t.Parallel()

	db := setupIntegrationDatabase(t)
	messageRepo := repository.NewMongoMessageRepository(db)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := messageRepo.EnsureIndexes(ctx, 86400); err != nil {
		t.Fatalf("failed to ensure indexes: %v", err)
	}

	baseMessage := &models.Message{
		TelegramMessageID: 10001,
		ChatID:            -20001,
		UserID:            30001,
		MessageType:       models.MessageTypeText,
		Text:              "original message",
		SentAt:            time.Now().Add(-2 * time.Minute).UTC(),
	}

	if err := messageRepo.CreateMessage(ctx, baseMessage); err != nil {
		t.Fatalf("failed to create base message: %v", err)
	}

	created, err := messageRepo.GetByTelegramID(ctx, baseMessage.TelegramMessageID, baseMessage.ChatID)
	if err != nil {
		t.Fatalf("failed to query base message: %v", err)
	}
	if created.Text != "original message" {
		t.Fatalf("unexpected text: got %q, want %q", created.Text, "original message")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected created_at and updated_at to be set")
	}

	editedAt := time.Now().UTC().Truncate(time.Second)
	if err := messageRepo.UpdateMessageEdit(
		ctx,
		baseMessage.TelegramMessageID,
		baseMessage.ChatID,
		"edited message",
		editedAt,
	); err != nil {
		t.Fatalf("failed to update message edit: %v", err)
	}

	updated, err := messageRepo.GetByTelegramID(ctx, baseMessage.TelegramMessageID, baseMessage.ChatID)
	if err != nil {
		t.Fatalf("failed to query updated message: %v", err)
	}
	if !updated.IsEdited {
		t.Fatalf("expected updated message to be marked edited")
	}
	if updated.Text != "edited message" {
		t.Fatalf("unexpected edited text: got %q, want %q", updated.Text, "edited message")
	}
	if updated.EditedAt == nil || updated.EditedAt.Unix() != editedAt.Unix() {
		t.Fatalf("unexpected edited_at: got %v, want unix=%d", updated.EditedAt, editedAt.Unix())
	}

	photoMessage := &models.Message{
		TelegramMessageID: 10002,
		ChatID:            baseMessage.ChatID,
		UserID:            30002,
		MessageType:       models.MessageTypePhoto,
		Caption:           "second message",
		SentAt:            time.Now().Add(-1 * time.Minute).UTC(),
	}
	if err := messageRepo.CreateMessage(ctx, photoMessage); err != nil {
		t.Fatalf("failed to create second message: %v", err)
	}

	messages, err := messageRepo.ListMessagesByChat(ctx, baseMessage.ChatID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("unexpected message count: got %d, want %d", len(messages), 2)
	}
	if messages[0].TelegramMessageID != photoMessage.TelegramMessageID {
		t.Fatalf(
			"expected newest message first, got id=%d want id=%d",
			messages[0].TelegramMessageID,
			photoMessage.TelegramMessageID,
		)
	}

	counts, err := messageRepo.CountMessagesByType(ctx, baseMessage.ChatID)
	if err != nil {
		t.Fatalf("failed to count messages by type: %v", err)
	}
	if counts[models.MessageTypeText] != 1 {
		t.Fatalf("unexpected text count: got %d, want %d", counts[models.MessageTypeText], 1)
	}
	if counts[models.MessageTypePhoto] != 1 {
		t.Fatalf("unexpected photo count: got %d, want %d", counts[models.MessageTypePhoto], 1)
	}
}

func setupIntegrationDatabase(t *testing.T) *mongodriver.Database {
	t.Helper()

	uri := envOrDefault("MONGO_URI", "mongodb://localhost:27017")
	baseDatabase := envOrDefault("TEST_DATABASE", "test_telegram_bot")
	databaseName := fmt.Sprintf("%s_%d", baseDatabase, time.Now().UnixNano())

	client, err := mongoclient.NewClient(mongoclient.Config{
		URI:      uri,
		Database: databaseName,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		if isCIEnvironment() {
			t.Fatalf("failed to connect MongoDB in CI: %v", err)
		}
		t.Skipf("MongoDB is not available locally, skip integration test: %v", err)
		return nil
	}

	db := client.Database()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := db.Drop(ctx); err != nil {
			t.Errorf("failed to drop integration database %s: %v", databaseName, err)
		}
		if err := client.Close(ctx); err != nil {
			t.Errorf("failed to close MongoDB connection: %v", err)
		}
	})

	return db
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func isCIEnvironment() bool {
	return os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true"
}
