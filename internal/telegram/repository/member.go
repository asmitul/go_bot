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
	memberEventsCollection = "member_events"
	joinRequestsCollection = "join_requests"
)

// MongoMemberRepository MongoDB 成员仓库实现
type MongoMemberRepository struct {
	db *mongo.Database
}

// NewMongoMemberRepository 创建成员仓库实例
func NewMongoMemberRepository(db *mongo.Database) MemberRepository {
	return &MongoMemberRepository{db: db}
}

// RecordEvent 记录成员事件
func (r *MongoMemberRepository) RecordEvent(ctx context.Context, event *models.ChatMemberEvent) error {
	if event.ChatID == 0 || event.UserID == 0 {
		return fmt.Errorf("chat_id and user_id are required")
	}

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	collection := r.db.Collection(memberEventsCollection)
	_, err := collection.InsertOne(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to record member event: %w", err)
	}

	return nil
}

// CreateJoinRequest 创建入群请求
func (r *MongoMemberRepository) CreateJoinRequest(ctx context.Context, request *models.JoinRequest) error {
	if request.ChatID == 0 || request.UserID == 0 {
		return fmt.Errorf("chat_id and user_id are required")
	}

	now := time.Now().UTC()
	if request.CreatedAt.IsZero() {
		request.CreatedAt = now
	}
	request.UpdatedAt = now

	collection := r.db.Collection(joinRequestsCollection)
	_, err := collection.InsertOne(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to create join request: %w", err)
	}

	return nil
}

// UpdateJoinRequestStatus 更新入群请求状态
func (r *MongoMemberRepository) UpdateJoinRequestStatus(ctx context.Context, requestID, reviewerID int64, status, note string) error {
	collection := r.db.Collection(joinRequestsCollection)

	filter := bson.M{
		"chat_id": requestID, // 这里简化处理，实际应该用 ObjectID
		"user_id": reviewerID,
		"status":  models.JoinRequestStatusPending,
	}

	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"reviewed_by": reviewerID,
			"reviewed_at": now,
			"review_note": note,
			"updated_at":  now,
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update join request status: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("join request not found or already processed")
	}

	return nil
}

// GetPendingRequests 获取待审批的入群请求
func (r *MongoMemberRepository) GetPendingRequests(ctx context.Context, chatID int64) ([]*models.JoinRequest, error) {
	collection := r.db.Collection(joinRequestsCollection)

	filter := bson.M{
		"chat_id": chatID,
		"status":  models.JoinRequestStatusPending,
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}) // 按时间升序

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending requests: %w", err)
	}
	defer cursor.Close(ctx)

	var requests []*models.JoinRequest
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, fmt.Errorf("failed to decode requests: %w", err)
	}

	return requests, nil
}

// GetJoinRequestByUser 根据用户和群组获取入群请求
func (r *MongoMemberRepository) GetJoinRequestByUser(ctx context.Context, chatID, userID int64) (*models.JoinRequest, error) {
	collection := r.db.Collection(joinRequestsCollection)

	filter := bson.M{
		"chat_id": chatID,
		"user_id": userID,
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}}) // 获取最新的

	var request models.JoinRequest
	err := collection.FindOne(ctx, filter, opts).Decode(&request)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("join request not found")
		}
		return nil, fmt.Errorf("failed to get join request: %w", err)
	}

	return &request, nil
}

// GetChatEvents 获取群组的成员事件历史
func (r *MongoMemberRepository) GetChatEvents(ctx context.Context, chatID int64, limit int) ([]*models.ChatMemberEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(memberEventsCollection)

	filter := bson.M{"chat_id": chatID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat events: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*models.ChatMemberEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("failed to decode events: %w", err)
	}

	return events, nil
}

// GetUserEvents 获取用户的成员事件历史
func (r *MongoMemberRepository) GetUserEvents(ctx context.Context, userID int64, limit int) ([]*models.ChatMemberEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(memberEventsCollection)

	filter := bson.M{"user_id": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get user events: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*models.ChatMemberEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("failed to decode events: %w", err)
	}

	return events, nil
}

// CountChatMembers 统计群组当前成员数（通过事件推算）
func (r *MongoMemberRepository) CountChatMembers(ctx context.Context, chatID int64) (int64, error) {
	// 简化实现：统计最近加入但未离开的成员
	collection := r.db.Collection(memberEventsCollection)

	// 聚合查询：获取每个用户的最新事件
	pipeline := []bson.M{
		{"$match": bson.M{"chat_id": chatID}},
		{"$sort": bson.M{"created_at": -1}},
		{"$group": bson.M{
			"_id":         "$user_id",
			"last_event":  bson.M{"$first": "$event_type"},
			"last_status": bson.M{"$first": "$new_status"},
		}},
		{"$match": bson.M{
			"last_status": bson.M{"$in": []string{models.MemberStatusMember, models.MemberStatusAdmin, models.MemberStatusCreator}},
		}},
		{"$count": "total"},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("failed to count members: %w", err)
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err := cursor.All(ctx, &result); err != nil {
		return 0, fmt.Errorf("failed to decode count: %w", err)
	}

	if len(result) == 0 {
		return 0, nil
	}

	total, ok := result[0]["total"].(int32)
	if !ok {
		return 0, nil
	}

	return int64(total), nil
}

// EnsureIndexes 确保索引存在
func (r *MongoMemberRepository) EnsureIndexes(ctx context.Context) error {
	// member_events 集合索引
	eventsCollection := r.db.Collection(memberEventsCollection)
	eventsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "event_type", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err := eventsCollection.Indexes().CreateMany(ctx, eventsIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for member_events: %w", err)
	}

	// join_requests 集合索引
	requestsCollection := r.db.Collection(joinRequestsCollection)
	requestsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "user_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "created_at", Value: 1},
			},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err = requestsCollection.Indexes().CreateMany(ctx, requestsIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for join_requests: %w", err)
	}

	return nil
}
