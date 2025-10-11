package repository

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	pollsCollection        = "polls"
	pollAnswersCollection  = "poll_answers"
)

// MongoPollRepository MongoDB 投票仓库实现
type MongoPollRepository struct {
	db *mongo.Database
}

// NewMongoPollRepository 创建投票仓库实例
func NewMongoPollRepository(db *mongo.Database) PollRepository {
	return &MongoPollRepository{db: db}
}

// CreatePoll 创建投票记录
func (r *MongoPollRepository) CreatePoll(ctx context.Context, poll *models.PollRecord) error {
	if poll.PollID == "" {
		return fmt.Errorf("poll_id is required")
	}

	now := time.Now().UTC()
	if poll.CreatedAt.IsZero() {
		poll.CreatedAt = now
	}
	poll.UpdatedAt = now

	collection := r.db.Collection(pollsCollection)
	_, err := collection.InsertOne(ctx, poll)
	if err != nil {
		return fmt.Errorf("failed to create poll: %w", err)
	}

	return nil
}

// UpdatePoll 更新投票状态
func (r *MongoPollRepository) UpdatePoll(ctx context.Context, poll *models.PollRecord) error {
	collection := r.db.Collection(pollsCollection)

	filter := bson.M{"poll_id": poll.PollID}
	poll.UpdatedAt = time.Now().UTC()

	update := bson.M{
		"$set": poll,
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update poll: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("poll not found")
	}

	return nil
}

// GetPollByID 根据 Poll ID 获取投票
func (r *MongoPollRepository) GetPollByID(ctx context.Context, pollID string) (*models.PollRecord, error) {
	collection := r.db.Collection(pollsCollection)

	filter := bson.M{"poll_id": pollID}

	var poll models.PollRecord
	err := collection.FindOne(ctx, filter).Decode(&poll)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	return &poll, nil
}

// RecordAnswer 记录投票回答
func (r *MongoPollRepository) RecordAnswer(ctx context.Context, answer *models.PollAnswer) error {
	if answer.PollID == "" || answer.UserID == 0 {
		return fmt.Errorf("poll_id and user_id are required")
	}

	if answer.CreatedAt.IsZero() {
		answer.CreatedAt = time.Now().UTC()
	}

	collection := r.db.Collection(pollAnswersCollection)

	// 使用 upsert 避免重复投票（更新最新的选择）
	filter := bson.M{
		"poll_id": answer.PollID,
		"user_id": answer.UserID,
	}

	update := bson.M{
		"$set": answer,
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to record answer: %w", err)
	}

	return nil
}

// GetPollAnswers 获取投票的所有回答
func (r *MongoPollRepository) GetPollAnswers(ctx context.Context, pollID string) ([]*models.PollAnswer, error) {
	collection := r.db.Collection(pollAnswersCollection)

	filter := bson.M{"poll_id": pollID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll answers: %w", err)
	}
	defer cursor.Close(ctx)

	var answers []*models.PollAnswer
	if err := cursor.All(ctx, &answers); err != nil {
		return nil, fmt.Errorf("failed to decode answers: %w", err)
	}

	return answers, nil
}

// GetUserPolls 获取用户创建的投票列表
func (r *MongoPollRepository) GetUserPolls(ctx context.Context, userID int64, limit int) ([]*models.PollRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(pollsCollection)

	filter := bson.M{"created_by": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get user polls: %w", err)
	}
	defer cursor.Close(ctx)

	var polls []*models.PollRecord
	if err := cursor.All(ctx, &polls); err != nil {
		return nil, fmt.Errorf("failed to decode polls: %w", err)
	}

	return polls, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoPollRepository) EnsureIndexes(ctx context.Context) error {
	// polls 索引
	pollsCollection := r.db.Collection(pollsCollection)
	pollsIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "poll_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "chat_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_by", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err := pollsCollection.Indexes().CreateMany(ctx, pollsIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for polls: %w", err)
	}

	// poll_answers 索引
	answersCollection := r.db.Collection(pollAnswersCollection)
	answersIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "poll_id", Value: 1},
				{Key: "user_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "poll_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: 1}},
		},
	}

	_, err = answersCollection.Indexes().CreateMany(ctx, answersIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for poll_answers: %w", err)
	}

	return nil
}
