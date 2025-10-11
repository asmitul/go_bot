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
	inlineQueriesCollection      = "inline_queries"
	chosenInlineResultsCollection = "chosen_inline_results"
)

// MongoInlineRepository MongoDB 内联查询仓库实现
type MongoInlineRepository struct {
	db *mongo.Database
}

// NewMongoInlineRepository 创建内联查询仓库实例
func NewMongoInlineRepository(db *mongo.Database) InlineRepository {
	return &MongoInlineRepository{db: db}
}

// LogQuery 记录内联查询
func (r *MongoInlineRepository) LogQuery(ctx context.Context, query *models.InlineQueryLog) error {
	if query.QueryID == "" {
		return fmt.Errorf("query_id is required")
	}

	if query.CreatedAt.IsZero() {
		query.CreatedAt = time.Now().UTC()
	}

	collection := r.db.Collection(inlineQueriesCollection)
	_, err := collection.InsertOne(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to log inline query: %w", err)
	}

	return nil
}

// LogChosenResult 记录内联结果选择
func (r *MongoInlineRepository) LogChosenResult(ctx context.Context, result *models.ChosenInlineResultLog) error {
	if result.ResultID == "" {
		return fmt.Errorf("result_id is required")
	}

	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	}

	collection := r.db.Collection(chosenInlineResultsCollection)
	_, err := collection.InsertOne(ctx, result)
	if err != nil {
		return fmt.Errorf("failed to log chosen result: %w", err)
	}

	return nil
}

// GetUserQueries 获取用户的内联查询历史
func (r *MongoInlineRepository) GetUserQueries(ctx context.Context, userID int64, limit int) ([]*models.InlineQueryLog, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(inlineQueriesCollection)

	filter := bson.M{"user_id": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get user queries: %w", err)
	}
	defer cursor.Close(ctx)

	var queries []*models.InlineQueryLog
	if err := cursor.All(ctx, &queries); err != nil {
		return nil, fmt.Errorf("failed to decode queries: %w", err)
	}

	return queries, nil
}

// GetPopularQueries 获取热门查询（按频率）
func (r *MongoInlineRepository) GetPopularQueries(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}

	collection := r.db.Collection(inlineQueriesCollection)

	// 聚合查询：按 query 分组并计数
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id":   "$query",
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": limit},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular queries: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	queries := make([]string, 0, len(results))
	for _, result := range results {
		if query, ok := result["_id"].(string); ok {
			queries = append(queries, query)
		}
	}

	return queries, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoInlineRepository) EnsureIndexes(ctx context.Context) error {
	// inline_queries 索引
	queriesCollection := r.db.Collection(inlineQueriesCollection)
	queriesIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "query_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "query", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err := queriesCollection.Indexes().CreateMany(ctx, queriesIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for inline_queries: %w", err)
	}

	// chosen_inline_results 索引
	resultsCollection := r.db.Collection(chosenInlineResultsCollection)
	resultsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "result_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err = resultsCollection.Indexes().CreateMany(ctx, resultsIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for chosen_inline_results: %w", err)
	}

	return nil
}
