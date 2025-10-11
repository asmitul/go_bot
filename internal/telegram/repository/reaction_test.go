//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"go_bot/internal/telegram/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupReactionTestDB(t *testing.T) (*mongo.Database, func()) {
	ctx := context.Background()
	mongoURI := "mongodb://localhost:27017"
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	dbName := "test_reaction_" + time.Now().Format("20060102150405")
	db := client.Database(dbName)

	cleanup := func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}

	return db, cleanup
}

func TestReactionRepository_RecordReaction(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试记录反应
	reaction := &models.MessageReactionRecord{
		ChatID:    -1001234567890,
		MessageID: 100,
		UserID:    12345,
		Username:  "testuser",
		Reactions: []models.Reaction{
			{Type: "emoji", Emoji: "👍"},
			{Type: "emoji", Emoji: "❤️"},
		},
	}

	err = repo.RecordReaction(ctx, reaction)
	assert.NoError(t, err)

	// 验证反应已记录
	reactions, err := repo.GetMessageReactions(ctx, -1001234567890, 100)
	require.NoError(t, err)
	assert.Len(t, reactions, 1)
	assert.Len(t, reactions[0].Reactions, 2)
	assert.Equal(t, "👍", reactions[0].Reactions[0].Emoji)
}

func TestReactionRepository_RecordReaction_Upsert(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	chatID := int64(-1001234567890)
	messageID := int64(200)
	userID := int64(54321)

	// 第一次记录反应
	reaction := &models.MessageReactionRecord{
		ChatID:    chatID,
		MessageID: messageID,
		UserID:    userID,
		Username:  "testuser",
		Reactions: []models.Reaction{
			{Type: "emoji", Emoji: "👍"},
		},
	}
	err = repo.RecordReaction(ctx, reaction)
	require.NoError(t, err)

	// 第二次记录同一用户的反应（更新）
	reaction.Reactions = []models.Reaction{
		{Type: "emoji", Emoji: "❤️"},
		{Type: "emoji", Emoji: "🔥"},
	}
	err = repo.RecordReaction(ctx, reaction)
	assert.NoError(t, err)

	// 验证只有一条记录，且反应已更新
	reactions, err := repo.GetMessageReactions(ctx, chatID, messageID)
	require.NoError(t, err)
	assert.Len(t, reactions, 1)
	assert.Len(t, reactions[0].Reactions, 2)
	assert.Equal(t, "❤️", reactions[0].Reactions[0].Emoji)
}

func TestReactionRepository_UpdateReactionCount(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试更新反应统计
	count := &models.MessageReactionCountRecord{
		ChatID:    -1001234567890,
		MessageID: 300,
		ReactionCounts: []models.ReactionCount{
			{
				Reaction: models.Reaction{Type: "emoji", Emoji: "👍"},
				Count:    5,
			},
			{
				Reaction: models.Reaction{Type: "emoji", Emoji: "❤️"},
				Count:    3,
			},
		},
		TotalCount: 8,
	}

	err = repo.UpdateReactionCount(ctx, count)
	assert.NoError(t, err)

	// 验证统计已更新
	retrieved, err := repo.GetReactionCount(ctx, -1001234567890, 300)
	require.NoError(t, err)
	assert.Equal(t, 8, retrieved.TotalCount)
	assert.Len(t, retrieved.ReactionCounts, 2)
}

func TestReactionRepository_GetMessageReactions(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	chatID := int64(-1001234567890)
	messageID := int64(400)

	// 记录多个用户的反应
	for i := 0; i < 3; i++ {
		reaction := &models.MessageReactionRecord{
			ChatID:    chatID,
			MessageID: messageID,
			UserID:    int64(10000 + i),
			Username:  "user" + string(rune(i+'A')),
			Reactions: []models.Reaction{
				{Type: "emoji", Emoji: "👍"},
			},
		}
		err = repo.RecordReaction(ctx, reaction)
		require.NoError(t, err)
	}

	// 获取消息的所有反应
	reactions, err := repo.GetMessageReactions(ctx, chatID, messageID)
	require.NoError(t, err)
	assert.Len(t, reactions, 3)
}

func TestReactionRepository_GetTopReactedMessages(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	chatID := int64(-1001234567890)

	// 创建多条消息的反应统计
	counts := []struct {
		messageID  int64
		totalCount int
	}{
		{500, 10},
		{501, 25},
		{502, 5},
		{503, 15},
	}

	for _, c := range counts {
		count := &models.MessageReactionCountRecord{
			ChatID:    chatID,
			MessageID: c.messageID,
			ReactionCounts: []models.ReactionCount{
				{
					Reaction: models.Reaction{Type: "emoji", Emoji: "👍"},
					Count:    c.totalCount,
				},
			},
			TotalCount: c.totalCount,
		}
		err = repo.UpdateReactionCount(ctx, count)
		require.NoError(t, err)
	}

	// 获取反应最多的消息（Top 3）
	topMessages, err := repo.GetTopReactedMessages(ctx, chatID, 3)
	require.NoError(t, err)
	assert.Len(t, topMessages, 3)

	// 验证排序正确（降序）
	assert.Equal(t, int64(501), topMessages[0].MessageID)
	assert.Equal(t, 25, topMessages[0].TotalCount)
	assert.Equal(t, int64(503), topMessages[1].MessageID)
	assert.Equal(t, 15, topMessages[1].TotalCount)
	assert.Equal(t, int64(500), topMessages[2].MessageID)
	assert.Equal(t, 10, topMessages[2].TotalCount)
}

func TestReactionRepository_GetReactionCount_NotFound(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	// 查询不存在的反应统计
	_, err := repo.GetReactionCount(ctx, -1001234567890, 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestReactionRepository_RecordReaction_EmptyReactions(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 记录空反应列表（移除所有反应）
	reaction := &models.MessageReactionRecord{
		ChatID:    -1001234567890,
		MessageID: 600,
		UserID:    12345,
		Username:  "testuser",
		Reactions: []models.Reaction{},
	}

	err = repo.RecordReaction(ctx, reaction)
	assert.NoError(t, err)

	// 验证记录已保存
	reactions, err := repo.GetMessageReactions(ctx, -1001234567890, 600)
	require.NoError(t, err)
	assert.Len(t, reactions, 1)
	assert.Empty(t, reactions[0].Reactions)
}

func TestReactionRepository_MultipleReactionTypes(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试多种反应类型
	reaction := &models.MessageReactionRecord{
		ChatID:    -1001234567890,
		MessageID: 700,
		UserID:    12345,
		Username:  "testuser",
		Reactions: []models.Reaction{
			{Type: "emoji", Emoji: "👍"},
			{Type: "emoji", Emoji: "❤️"},
			{Type: "custom_emoji", Emoji: ""},
		},
	}

	err = repo.RecordReaction(ctx, reaction)
	assert.NoError(t, err)

	// 验证反应已记录
	reactions, err := repo.GetMessageReactions(ctx, -1001234567890, 700)
	require.NoError(t, err)
	assert.Len(t, reactions, 1)
	assert.Len(t, reactions[0].Reactions, 3)
	assert.Equal(t, "emoji", reactions[0].Reactions[0].Type)
	assert.Equal(t, "custom_emoji", reactions[0].Reactions[2].Type)
}

func TestReactionRepository_EnsureIndexes(t *testing.T) {
	db, cleanup := setupReactionTestDB(t)
	defer cleanup()

	repo := NewMongoReactionRepository(db)
	ctx := context.Background()

	// 测试索引创建
	err := repo.EnsureIndexes(ctx)
	assert.NoError(t, err)

	// 再次调用不应报错
	err = repo.EnsureIndexes(ctx)
	assert.NoError(t, err)
}
