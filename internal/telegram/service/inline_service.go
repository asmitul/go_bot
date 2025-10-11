package service

import (
	"context"
	"fmt"
	"strings"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// inlineService 内联查询服务实现
type inlineService struct {
	inlineRepo repository.InlineRepository
}

// NewInlineService 创建内联查询服务
func NewInlineService(inlineRepo repository.InlineRepository) InlineService {
	return &inlineService{
		inlineRepo: inlineRepo,
	}
}

// HandleInlineQuery 处理内联查询
func (s *inlineService) HandleInlineQuery(ctx context.Context, query *models.InlineQueryLog) error {
	// 验证查询内容
	if !s.ValidateQuery(query.Query) {
		logger.L().WithField("query", query.Query).Warn("内联查询包含非法内容")
		return fmt.Errorf("查询内容不合法")
	}

	// 记录查询日志
	if err := s.inlineRepo.LogQuery(ctx, query); err != nil {
		logger.L().WithError(err).Error("记录内联查询失败")
		return fmt.Errorf("记录查询失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"query_id": query.QueryID,
		"user_id":  query.UserID,
		"query":    query.Query,
	}).Info("内联查询处理成功")

	return nil
}

// HandleChosenResult 处理内联结果选择
func (s *inlineService) HandleChosenResult(ctx context.Context, result *models.ChosenInlineResultLog) error {
	// 记录选择日志
	if err := s.inlineRepo.LogChosenResult(ctx, result); err != nil {
		logger.L().WithError(err).Error("记录内联结果选择失败")
		return fmt.Errorf("记录结果选择失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"result_id": result.ResultID,
		"user_id":   result.UserID,
		"query":     result.Query,
	}).Info("内联结果选择记录成功")

	return nil
}

// GetUserQueryHistory 获取用户内联查询历史
func (s *inlineService) GetUserQueryHistory(ctx context.Context, userID int64, limit int) ([]*models.InlineQueryLog, error) {
	queries, err := s.inlineRepo.GetUserQueries(ctx, userID, limit)
	if err != nil {
		logger.L().WithError(err).WithField("user_id", userID).Error("获取用户查询历史失败")
		return nil, fmt.Errorf("获取查询历史失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"user_id": userID,
		"count":   len(queries),
	}).Info("查询历史获取成功")

	return queries, nil
}

// GetPopularQueries 获取热门查询
func (s *inlineService) GetPopularQueries(ctx context.Context, limit int) ([]string, error) {
	queries, err := s.inlineRepo.GetPopularQueries(ctx, limit)
	if err != nil {
		logger.L().WithError(err).Error("获取热门查询失败")
		return nil, fmt.Errorf("获取热门查询失败: %w", err)
	}

	logger.L().WithField("count", len(queries)).Info("热门查询获取成功")

	return queries, nil
}

// ValidateQuery 验证查询内容是否合法
func (s *inlineService) ValidateQuery(query string) bool {
	// 基本验证规则
	if len(query) > 256 {
		return false
	}

	// 检查是否包含敏感词（示例）
	bannedWords := []string{"spam", "abuse", "malicious"}
	queryLower := strings.ToLower(query)
	for _, word := range bannedWords {
		if strings.Contains(queryLower, word) {
			return false
		}
	}

	return true
}
