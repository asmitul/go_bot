package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

const defaultBalanceAlertLimit = 3

// MongoUpstreamBalanceRepository 上游群余额数据访问层（MongoDB 实现）
type MongoUpstreamBalanceRepository struct {
	balanceColl  *mongo.Collection
	logColl      *mongo.Collection
	snapshotColl *mongo.Collection
}

// NewMongoUpstreamBalanceRepository 创建仓储实例
func NewMongoUpstreamBalanceRepository(db *mongo.Database) UpstreamBalanceRepository {
	return &MongoUpstreamBalanceRepository{
		balanceColl:  db.Collection("upstream_balances"),
		logColl:      db.Collection("upstream_balance_logs"),
		snapshotColl: db.Collection("upstream_settlement_snapshots"),
	}
}

// Get 获取或创建余额记录
func (r *MongoUpstreamBalanceRepository) Get(ctx context.Context, groupID int64) (*models.UpstreamBalance, error) {
	now := time.Now()
	filter := balanceFilter(groupID)
	update := bson.M{
		"$setOnInsert": bson.M{
			"chat_id":              groupID,
			"group_id":             groupID,
			"balance":              0.0,
			"min_balance":          0.0,
			"alert_limit_per_hour": defaultBalanceAlertLimit,
			"created_at":           now,
		},
		"$set": bson.M{
			"updated_at": now,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var balance models.UpstreamBalance
	if err := r.balanceColl.FindOneAndUpdate(ctx, filter, update, opts).Decode(&balance); err != nil {
		return nil, fmt.Errorf("failed to get upstream balance: %w", err)
	}

	return &balance, nil
}

// Adjust 调整余额并写入日志（事务）
func (r *MongoUpstreamBalanceRepository) Adjust(
	ctx context.Context,
	groupID int64,
	delta float64,
	operatorID int64,
	remark string,
	opType models.BalanceOperationType,
	operationID string,
	metadata map[string]string,
) (*models.UpstreamBalance, error) {
	client := r.balanceColl.Database().Client()
	session, err := client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	txnOpts := options.Transaction().SetWriteConcern(writeconcern.Majority())
	result, err := session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		if operationID != "" {
			if existing, err := r.findLogByOperation(sc, groupID, operationID); err == nil && existing != nil {
				return r.Get(sc, groupID)
			}
		}

		now := time.Now()
		filter := balanceFilter(groupID)
		update := bson.M{
			"$inc": bson.M{
				"balance": delta,
			},
			"$set": bson.M{
				"updated_at": now,
			},
			"$setOnInsert": bson.M{
				"chat_id":              groupID,
				"group_id":             groupID,
				"min_balance":          0.0,
				"alert_limit_per_hour": defaultBalanceAlertLimit,
				"created_at":           now,
			},
		}

		opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
		var balance models.UpstreamBalance
		if err := r.balanceColl.FindOneAndUpdate(sc, filter, update, opts).Decode(&balance); err != nil {
			return nil, fmt.Errorf("update balance failed: %w", err)
		}

		logEntry := &models.UpstreamBalanceLog{
			GroupID:     groupID,
			OperatorID:  operatorID,
			Delta:       delta,
			Balance:     balance.Balance,
			Type:        opType,
			Remark:      remark,
			OperationID: operationID,
			CreatedAt:   now,
			Metadata:    metadata,
		}

		if _, err := r.logColl.InsertOne(sc, logEntry); err != nil {
			return nil, fmt.Errorf("insert balance log failed: %w", err)
		}

		return &balance, nil
	}, txnOpts)

	if err != nil {
		if isTransactionNotSupported(err) {
			return r.adjustWithoutTransaction(ctx, groupID, delta, operatorID, remark, opType, operationID, metadata)
		}
		return nil, fmt.Errorf("balance adjust transaction failed: %w", err)
	}

	balance, _ := result.(*models.UpstreamBalance)
	if balance == nil {
		return nil, errors.New("balance adjust transaction returned nil")
	}

	return balance, nil
}

func (r *MongoUpstreamBalanceRepository) adjustWithoutTransaction(
	ctx context.Context,
	groupID int64,
	delta float64,
	operatorID int64,
	remark string,
	opType models.BalanceOperationType,
	operationID string,
	metadata map[string]string,
) (*models.UpstreamBalance, error) {
	if operationID != "" {
		if existing, err := r.findLogByOperation(ctx, groupID, operationID); err == nil && existing != nil {
			return r.Get(ctx, groupID)
		}
	}

	now := time.Now()
	filter := balanceFilter(groupID)
	update := bson.M{
		"$inc": bson.M{
			"balance": delta,
		},
		"$set": bson.M{
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"chat_id":              groupID,
			"group_id":             groupID,
			"min_balance":          0.0,
			"alert_limit_per_hour": defaultBalanceAlertLimit,
			"created_at":           now,
		},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var balance models.UpstreamBalance
	if err := r.balanceColl.FindOneAndUpdate(ctx, filter, update, opts).Decode(&balance); err != nil {
		return nil, fmt.Errorf("update balance failed (non-txn): %w", err)
	}

	logEntry := &models.UpstreamBalanceLog{
		GroupID:     groupID,
		OperatorID:  operatorID,
		Delta:       delta,
		Balance:     balance.Balance,
		Type:        opType,
		Remark:      remark,
		OperationID: operationID,
		CreatedAt:   now,
		Metadata:    metadata,
	}

	if _, err := r.logColl.InsertOne(ctx, logEntry); err != nil {
		return nil, fmt.Errorf("insert balance log failed (non-txn): %w", err)
	}

	return &balance, nil
}

// SetMinBalance 更新最低余额阈值并写入日志
func (r *MongoUpstreamBalanceRepository) SetMinBalance(ctx context.Context, groupID int64, threshold float64, operatorID int64) (*models.UpstreamBalance, error) {
	return r.updateSettings(ctx, groupID, bson.M{"min_balance": threshold}, operatorID, models.BalanceOpSetMinBalance, fmt.Sprintf("设置最低余额 %.2f", threshold))
}

// SetAlertLimit 更新告警频率并写入日志
func (r *MongoUpstreamBalanceRepository) SetAlertLimit(ctx context.Context, groupID int64, limit int, operatorID int64) (*models.UpstreamBalance, error) {
	return r.updateSettings(ctx, groupID, bson.M{"alert_limit_per_hour": limit}, operatorID, models.BalanceOpAlertLimit, fmt.Sprintf("设置告警频率 %d/h", limit))
}

func (r *MongoUpstreamBalanceRepository) updateSettings(ctx context.Context, groupID int64, setFields bson.M, operatorID int64, opType models.BalanceOperationType, remark string) (*models.UpstreamBalance, error) {
	client := r.balanceColl.Database().Client()
	session, err := client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	txnOpts := options.Transaction().SetWriteConcern(writeconcern.Majority())
	result, err := session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		now := time.Now()
		filter := balanceFilter(groupID)
		update := bson.M{
			"$set": mergeBson(setFields, bson.M{
				"updated_at": now,
			}),
			"$setOnInsert": filterDefaults(bson.M{
				"chat_id":              groupID,
				"group_id":             groupID,
				"min_balance":          0.0,
				"alert_limit_per_hour": defaultBalanceAlertLimit,
				"created_at":           now,
			}, setFields),
		}

		opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
		var balance models.UpstreamBalance
		if err := r.balanceColl.FindOneAndUpdate(sc, filter, update, opts).Decode(&balance); err != nil {
			return nil, fmt.Errorf("update balance settings failed: %w", err)
		}

		logEntry := &models.UpstreamBalanceLog{
			GroupID:    groupID,
			OperatorID: operatorID,
			Delta:      0,
			Balance:    balance.Balance,
			Type:       opType,
			Remark:     remark,
			CreatedAt:  now,
		}
		if _, err := r.logColl.InsertOne(sc, logEntry); err != nil {
			return nil, fmt.Errorf("insert balance log failed: %w", err)
		}

		return &balance, nil
	}, txnOpts)

	if err != nil {
		if isTransactionNotSupported(err) {
			return r.updateSettingsWithoutTransaction(ctx, groupID, setFields, operatorID, opType, remark)
		}
		return nil, fmt.Errorf("balance settings transaction failed: %w", err)
	}

	balance, _ := result.(*models.UpstreamBalance)
	if balance == nil {
		return nil, errors.New("balance settings transaction returned nil")
	}
	return balance, nil
}

func (r *MongoUpstreamBalanceRepository) updateSettingsWithoutTransaction(ctx context.Context, groupID int64, setFields bson.M, operatorID int64, opType models.BalanceOperationType, remark string) (*models.UpstreamBalance, error) {
	now := time.Now()
	filter := balanceFilter(groupID)
	update := bson.M{
		"$set": mergeBson(setFields, bson.M{
			"updated_at": now,
		}),
		"$setOnInsert": filterDefaults(bson.M{
			"chat_id":              groupID,
			"group_id":             groupID,
			"min_balance":          0.0,
			"alert_limit_per_hour": defaultBalanceAlertLimit,
			"created_at":           now,
		}, setFields),
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var balance models.UpstreamBalance
	if err := r.balanceColl.FindOneAndUpdate(ctx, filter, update, opts).Decode(&balance); err != nil {
		return nil, fmt.Errorf("update balance settings failed (non-txn): %w", err)
	}

	logEntry := &models.UpstreamBalanceLog{
		GroupID:    groupID,
		OperatorID: operatorID,
		Delta:      0,
		Balance:    balance.Balance,
		Type:       opType,
		Remark:     remark,
		CreatedAt:  now,
	}
	if _, err := r.logColl.InsertOne(ctx, logEntry); err != nil {
		return nil, fmt.Errorf("insert balance log failed (non-txn): %w", err)
	}

	return &balance, nil
}

// ListAll 列出所有余额记录
func (r *MongoUpstreamBalanceRepository) ListAll(ctx context.Context) ([]*models.UpstreamBalance, error) {
	cursor, err := r.balanceColl.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("list balances failed: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var balances []*models.UpstreamBalance
	if err := cursor.All(ctx, &balances); err != nil {
		return nil, fmt.Errorf("decode balances failed: %w", err)
	}
	return balances, nil
}

// ListAdjustmentLogsByDateRange 查询指定日期范围内的人工出入账日志
func (r *MongoUpstreamBalanceRepository) ListAdjustmentLogsByDateRange(ctx context.Context, groupID int64, startTime, endTime time.Time) ([]*models.UpstreamBalanceLog, error) {
	filter := bson.M{
		"group_id": groupID,
		"type": bson.M{
			"$in": []models.BalanceOperationType{models.BalanceOpCredit, models.BalanceOpDebit},
		},
		"created_at": bson.M{
			"$gte": startTime,
			"$lt":  endTime,
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.logColl.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list adjustment logs failed: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var logs []*models.UpstreamBalanceLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("decode adjustment logs failed: %w", err)
	}
	return logs, nil
}

// CreateSettlementSnapshot 保存日结快照，已存在时返回原记录
func (r *MongoUpstreamBalanceRepository) CreateSettlementSnapshot(ctx context.Context, snapshot *models.UpstreamSettlementSnapshot) (*models.UpstreamSettlementSnapshot, error) {
	if snapshot == nil {
		return nil, errors.New("settlement snapshot is nil")
	}
	if snapshot.GroupID == 0 {
		return nil, errors.New("settlement snapshot group_id is required")
	}
	if strings.TrimSpace(snapshot.Date) == "" {
		return nil, errors.New("settlement snapshot date is required")
	}

	now := time.Now()
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = now
	}

	filter := bson.M{
		"group_id": snapshot.GroupID,
		"date":     strings.TrimSpace(snapshot.Date),
	}
	update := bson.M{
		"$setOnInsert": bson.M{
			"group_id":          snapshot.GroupID,
			"date":              strings.TrimSpace(snapshot.Date),
			"opening_prepaid":   snapshot.OpeningPrepaid,
			"settlement_amount": snapshot.SettlementAmount,
			"closing_prepaid":   snapshot.ClosingPrepaid,
			"operation_id":      snapshot.OperationID,
			"created_at":        snapshot.CreatedAt,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var saved models.UpstreamSettlementSnapshot
	if err := r.snapshotColl.FindOneAndUpdate(ctx, filter, update, opts).Decode(&saved); err != nil {
		return nil, fmt.Errorf("save settlement snapshot: %w", err)
	}
	return &saved, nil
}

// GetSettlementSnapshot 获取指定日期的日结快照
func (r *MongoUpstreamBalanceRepository) GetSettlementSnapshot(ctx context.Context, groupID int64, date string) (*models.UpstreamSettlementSnapshot, error) {
	filter := bson.M{
		"group_id": groupID,
		"date":     strings.TrimSpace(date),
	}

	var snapshot models.UpstreamSettlementSnapshot
	err := r.snapshotColl.FindOne(ctx, filter).Decode(&snapshot)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("get settlement snapshot: %w", err)
	}
	return &snapshot, nil
}

// EnsureIndexes 创建需要的索引
func (r *MongoUpstreamBalanceRepository) EnsureIndexes(ctx context.Context) error {
	balanceIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "group_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "updated_at", Value: -1}},
		},
	}

	if _, err := r.balanceColl.Indexes().CreateMany(ctx, balanceIndexes); err != nil {
		return fmt.Errorf("create balance indexes: %w", err)
	}

	logIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "type", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys:    bson.D{{Key: "operation_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	}

	if _, err := r.logColl.Indexes().CreateMany(ctx, logIndexes); err != nil {
		return fmt.Errorf("create balance log indexes: %w", err)
	}

	snapshotIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "date", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "operation_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	}
	if _, err := r.snapshotColl.Indexes().CreateMany(ctx, snapshotIndexes); err != nil {
		return fmt.Errorf("create settlement snapshot indexes: %w", err)
	}

	return nil
}

func (r *MongoUpstreamBalanceRepository) findLogByOperation(ctx context.Context, groupID int64, operationID string) (*models.UpstreamBalanceLog, error) {
	if operationID == "" {
		return nil, nil
	}

	filter := bson.M{
		"group_id":     groupID,
		"operation_id": operationID,
	}

	var log models.UpstreamBalanceLog
	err := r.logColl.FindOne(ctx, filter).Decode(&log)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

func mergeBson(a, b bson.M) bson.M {
	result := bson.M{}
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func isTransactionNotSupported(err error) bool {
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.Code == 20 || strings.EqualFold(cmdErr.Name, "IllegalOperation") {
			return true
		}
		if strings.Contains(strings.ToLower(cmdErr.Message), "transaction numbers are only allowed on a replica set") {
			return true
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "transaction numbers are only allowed on a replica set") {
		return true
	}
	return false
}

func balanceFilter(groupID int64) bson.M {
	return bson.M{
		"$or": []bson.M{
			{"group_id": groupID},
			{"chat_id": groupID},
		},
	}
}

func filterDefaults(defaults bson.M, setFields bson.M) bson.M {
	if len(defaults) == 0 {
		return defaults
	}
	if len(setFields) == 0 {
		return defaults
	}

	result := bson.M{}
	for k, v := range defaults {
		if _, exists := setFields[k]; exists {
			continue
		}
		result[k] = v
	}
	return result
}
