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

func setupPollTestDB(t *testing.T) (*mongo.Database, func()) {
	ctx := context.Background()
	mongoURI := "mongodb://localhost:27017"
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	dbName := "test_poll_" + time.Now().Format("20060102150405")
	db := client.Database(dbName)

	cleanup := func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}

	return db, cleanup
}

func TestPollRepository_CreatePoll(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试创建投票
	poll := &models.PollRecord{
		PollID:                "poll123",
		ChatID:                -1001234567890,
		MessageID:             999,
		Question:              "What is your favorite color?",
		Type:                  models.PollTypeRegular,
		AllowsMultipleAnswers: false,
		IsAnonymous:           true,
		IsClosed:              false,
		TotalVoterCount:       0,
		CreatedBy:             12345,
		Options: []models.PollOption{
			{Text: "Red", VoterCount: 0},
			{Text: "Blue", VoterCount: 0},
			{Text: "Green", VoterCount: 0},
		},
	}

	err = repo.CreatePoll(ctx, poll)
	assert.NoError(t, err)

	// 验证投票已创建
	retrieved, err := repo.GetPollByID(ctx, "poll123")
	require.NoError(t, err)
	assert.Equal(t, "poll123", retrieved.PollID)
	assert.Equal(t, "What is your favorite color?", retrieved.Question)
	assert.Len(t, retrieved.Options, 3)
}

func TestPollRepository_UpdatePoll(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 创建投票
	poll := &models.PollRecord{
		PollID:          "poll456",
		Question:        "Test Poll",
		Type:            models.PollTypeRegular,
		IsClosed:        false,
		TotalVoterCount: 0,
		Options: []models.PollOption{
			{Text: "Option 1", VoterCount: 0},
		},
	}
	err = repo.CreatePoll(ctx, poll)
	require.NoError(t, err)

	// 更新投票状态
	poll.IsClosed = true
	poll.TotalVoterCount = 5
	poll.Options[0].VoterCount = 5
	now := time.Now()
	poll.ClosedAt = &now

	err = repo.UpdatePoll(ctx, poll)
	assert.NoError(t, err)

	// 验证更新
	updated, err := repo.GetPollByID(ctx, "poll456")
	require.NoError(t, err)
	assert.True(t, updated.IsClosed)
	assert.Equal(t, 5, updated.TotalVoterCount)
	assert.NotNil(t, updated.ClosedAt)
}

func TestPollRepository_RecordAnswer(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试记录回答
	answer := &models.PollAnswer{
		PollID:    "poll789",
		UserID:    12345,
		Username:  "testuser",
		OptionIDs: []int{0},
	}

	err = repo.RecordAnswer(ctx, answer)
	assert.NoError(t, err)

	// 再次记录同一用户的回答（应该更新）
	answer.OptionIDs = []int{1}
	err = repo.RecordAnswer(ctx, answer)
	assert.NoError(t, err)

	// 验证只有一条记录
	answers, err := repo.GetPollAnswers(ctx, "poll789")
	require.NoError(t, err)
	assert.Len(t, answers, 1)
	assert.Equal(t, []int{1}, answers[0].OptionIDs)
}

func TestPollRepository_GetPollAnswers(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	pollID := "poll_multi"

	// 插入多个回答
	for i := 0; i < 3; i++ {
		answer := &models.PollAnswer{
			PollID:    pollID,
			UserID:    int64(10000 + i),
			Username:  "user" + string(rune(i+'A')),
			OptionIDs: []int{i % 2},
		}
		err = repo.RecordAnswer(ctx, answer)
		require.NoError(t, err)
	}

	// 获取所有回答
	answers, err := repo.GetPollAnswers(ctx, pollID)
	require.NoError(t, err)
	assert.Len(t, answers, 3)
}

func TestPollRepository_GetUserPolls(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	userID := int64(99999)

	// 创建多个投票
	for i := 0; i < 3; i++ {
		poll := &models.PollRecord{
			PollID:    "poll_user_" + string(rune(i+'A')),
			Question:  "Question " + string(rune(i+'A')),
			Type:      models.PollTypeRegular,
			CreatedBy: userID,
			Options: []models.PollOption{
				{Text: "Yes", VoterCount: 0},
				{Text: "No", VoterCount: 0},
			},
		}
		err = repo.CreatePoll(ctx, poll)
		require.NoError(t, err)
	}

	// 获取用户创建的投票
	polls, err := repo.GetUserPolls(ctx, userID, 10)
	require.NoError(t, err)
	assert.Len(t, polls, 3)

	// 测试限制
	polls, err = repo.GetUserPolls(ctx, userID, 2)
	require.NoError(t, err)
	assert.Len(t, polls, 2)
}

func TestPollRepository_GetPollByID_NotFound(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	// 查询不存在的投票
	_, err := repo.GetPollByID(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPollRepository_UpdatePoll_NotFound(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	// 更新不存在的投票
	poll := &models.PollRecord{
		PollID:   "nonexistent",
		Question: "Test",
		Type:     models.PollTypeRegular,
		Options:  []models.PollOption{{Text: "Option", VoterCount: 0}},
	}

	err := repo.UpdatePoll(ctx, poll)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPollRepository_CreateQuizPoll(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	err := repo.EnsureIndexes(ctx)
	require.NoError(t, err)

	// 测试创建测验类型投票
	quiz := &models.PollRecord{
		PollID:          "quiz123",
		Question:        "What is 2+2?",
		Type:            models.PollTypeQuiz,
		CorrectOptionID: 2,
		IsClosed:        false,
		Options: []models.PollOption{
			{Text: "3", VoterCount: 0},
			{Text: "5", VoterCount: 0},
			{Text: "4", VoterCount: 0},
		},
	}

	err = repo.CreatePoll(ctx, quiz)
	assert.NoError(t, err)

	// 验证测验已创建
	retrieved, err := repo.GetPollByID(ctx, "quiz123")
	require.NoError(t, err)
	assert.Equal(t, models.PollTypeQuiz, retrieved.Type)
	assert.Equal(t, 2, retrieved.CorrectOptionID)
}

func TestPollRepository_EnsureIndexes(t *testing.T) {
	db, cleanup := setupPollTestDB(t)
	defer cleanup()

	repo := NewMongoPollRepository(db)
	ctx := context.Background()

	// 测试索引创建
	err := repo.EnsureIndexes(ctx)
	assert.NoError(t, err)

	// 再次调用不应报错
	err = repo.EnsureIndexes(ctx)
	assert.NoError(t, err)
}
