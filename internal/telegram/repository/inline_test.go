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

func setupInlineTestDB(t *testing.T) (*mongo.Database, func()) {
	ctx := context.Background()
	mongoURI := "mongodb://localhost:27017"
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	dbName := "test_inline_" + time.Now().Format("20060102150405")
	db := client.Database(dbName)

	cleanup := func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}

	return db, cleanup
}

func TestInlineRepository_LogQuery(t *testing.T) {
	db, cleanup := setupInlineTestDB(t)
	defer cleanup()

	repo := NewMongoInlineRepository(db)
	ctx := context.Background()

	// 确保索引存在
	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试记录查询
	query := &models.InlineQueryLog{
		QueryID:  "query123",
		UserID:   12345,
		Username: "testuser",
		Query:    "test search",
		Offset:   "",
	}

	err = repo.LogQuery(ctx, query)
	assert.NoError(t, err)

	// 验证查询已记录
	queries, err := repo.GetUserQueries(ctx, 12345, 10)
	require.NoError(t, err)
	assert.Len(t, queries, 1)
	assert.Equal(t, "query123", queries[0].QueryID)
	assert.Equal(t, "test search", queries[0].Query)
}

func TestInlineRepository_LogChosenResult(t *testing.T) {
	db, cleanup := setupInlineTestDB(t)
	defer cleanup()

	repo := NewMongoInlineRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试记录选择结果
	result := &models.ChosenInlineResultLog{
		ResultID: "result456",
		UserID:   12345,
		Username: "testuser",
		Query:    "test search",
	}

	err = repo.LogChosenResult(ctx, result)
	assert.NoError(t, err)
}

func TestInlineRepository_GetPopularQueries(t *testing.T) {
	db, cleanup := setupInlineTestDB(t)
	defer cleanup()

	repo := NewMongoInlineRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 插入多个查询（相同查询内容）
	for i := 0; i < 5; i++ {
		query := &models.InlineQueryLog{
			QueryID:  "query" + time.Now().Format("150405.000000"),
			UserID:   int64(12345 + i),
			Username: "testuser",
			Query:    "popular search",
		}
		err = repo.LogQuery(ctx, query)
		require.NoError(t, err)
		time.Sleep(time.Millisecond) // 确保 QueryID 唯一
	}

	// 插入另一个不同的查询
	query2 := &models.InlineQueryLog{
		QueryID:  "query_other",
		UserID:   99999,
		Username: "otheruser",
		Query:    "rare search",
	}
	err = repo.LogQuery(ctx, query2)
	require.NoError(t, err)

	// 获取热门查询
	popular, err := repo.GetPopularQueries(ctx, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, popular)
	assert.Equal(t, "popular search", popular[0])
}

func TestInlineRepository_GetUserQueries(t *testing.T) {
	db, cleanup := setupInlineTestDB(t)
	defer cleanup()

	repo := NewMongoInlineRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	userID := int64(54321)

	// 插入多个查询
	for i := 0; i < 3; i++ {
		query := &models.InlineQueryLog{
			QueryID:  "query" + time.Now().Format("150405.000000"),
			UserID:   userID,
			Username: "testuser",
			Query:    "search " + string(rune(i+'A')),
		}
		err = repo.LogQuery(ctx, query)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
	}

	// 获取用户查询历史
	queries, err := repo.GetUserQueries(ctx, userID, 10)
	require.NoError(t, err)
	assert.Len(t, queries, 3)

	// 测试限制
	queries, err = repo.GetUserQueries(ctx, userID, 2)
	require.NoError(t, err)
	assert.Len(t, queries, 2)
}

func TestInlineRepository_EnsureIndexes(t *testing.T) {
	db, cleanup := setupInlineTestDB(t)
	defer cleanup()

	repo := NewMongoInlineRepository(db)
	ctx := context.Background()

	// 测试索引创建
	err := repo.EnsureIndexes(ctx)
	assert.NoError(t, err)

	// 再次调用不应报错
	err = repo.EnsureIndexes(ctx)
	assert.NoError(t, err)
}
