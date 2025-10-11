package service

import (
	"context"
	"fmt"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// pollService 投票服务实现
type pollService struct {
	pollRepo repository.PollRepository
}

// NewPollService 创建投票服务
func NewPollService(pollRepo repository.PollRepository) PollService {
	return &pollService{
		pollRepo: pollRepo,
	}
}

// HandlePollCreation 处理投票创建
func (s *pollService) HandlePollCreation(ctx context.Context, poll *models.PollRecord) error {
	// 验证投票参数
	if err := s.ValidatePoll(poll); err != nil {
		logger.L().WithError(err).Warn("投票参数验证失败")
		return err
	}

	// 创建投票记录
	if err := s.pollRepo.CreatePoll(ctx, poll); err != nil {
		logger.L().WithError(err).Error("创建投票记录失败")
		return fmt.Errorf("创建投票失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"poll_id":  poll.PollID,
		"question": poll.Question,
		"type":     poll.Type,
		"chat_id":  poll.ChatID,
	}).Info("投票创建成功")

	return nil
}

// HandlePollUpdate 处理投票更新
func (s *pollService) HandlePollUpdate(ctx context.Context, poll *models.PollRecord) error {
	// 更新投票状态
	if err := s.pollRepo.UpdatePoll(ctx, poll); err != nil {
		logger.L().WithError(err).Error("更新投票状态失败")
		return fmt.Errorf("更新投票失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"poll_id":           poll.PollID,
		"is_closed":         poll.IsClosed,
		"total_voter_count": poll.TotalVoterCount,
	}).Info("投票更新成功")

	return nil
}

// HandlePollAnswer 处理用户投票
func (s *pollService) HandlePollAnswer(ctx context.Context, answer *models.PollAnswer) error {
	// 验证投票是否存在
	poll, err := s.pollRepo.GetPollByID(ctx, answer.PollID)
	if err != nil {
		logger.L().WithError(err).WithField("poll_id", answer.PollID).Error("获取投票信息失败")
		return fmt.Errorf("投票不存在")
	}

	// 检查投票是否已关闭
	if poll.IsClosed {
		logger.L().WithField("poll_id", answer.PollID).Warn("尝试对已关闭的投票进行投票")
		return fmt.Errorf("投票已关闭")
	}

	// 如果是测验，检查答案是否正确
	if poll.IsQuiz() && len(answer.OptionIDs) > 0 {
		isCorrect := answer.OptionIDs[0] == poll.CorrectOptionID
		answer.IsCorrect = &isCorrect
	}

	// 记录投票回答
	if err := s.pollRepo.RecordAnswer(ctx, answer); err != nil {
		logger.L().WithError(err).Error("记录投票回答失败")
		return fmt.Errorf("记录投票失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"poll_id":    answer.PollID,
		"user_id":    answer.UserID,
		"option_ids": answer.OptionIDs,
	}).Info("投票回答记录成功")

	return nil
}

// GetPollResults 获取投票结果
func (s *pollService) GetPollResults(ctx context.Context, pollID string) (*models.PollRecord, []*models.PollAnswer, error) {
	// 获取投票信息
	poll, err := s.pollRepo.GetPollByID(ctx, pollID)
	if err != nil {
		logger.L().WithError(err).WithField("poll_id", pollID).Error("获取投票信息失败")
		return nil, nil, fmt.Errorf("投票不存在")
	}

	// 获取所有回答
	answers, err := s.pollRepo.GetPollAnswers(ctx, pollID)
	if err != nil {
		logger.L().WithError(err).WithField("poll_id", pollID).Error("获取投票回答失败")
		return nil, nil, fmt.Errorf("获取回答失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"poll_id":      pollID,
		"answer_count": len(answers),
	}).Info("投票结果获取成功")

	return poll, answers, nil
}

// GetUserPolls 获取用户创建的投票
func (s *pollService) GetUserPolls(ctx context.Context, userID int64, limit int) ([]*models.PollRecord, error) {
	polls, err := s.pollRepo.GetUserPolls(ctx, userID, limit)
	if err != nil {
		logger.L().WithError(err).WithField("user_id", userID).Error("获取用户投票列表失败")
		return nil, fmt.Errorf("获取投票列表失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"user_id": userID,
		"count":   len(polls),
	}).Info("用户投票列表获取成功")

	return polls, nil
}

// ValidatePoll 验证投票参数是否合法
func (s *pollService) ValidatePoll(poll *models.PollRecord) error {
	// 验证问题
	if poll.Question == "" {
		return fmt.Errorf("投票问题不能为空")
	}
	if len(poll.Question) > 300 {
		return fmt.Errorf("投票问题过长（最多 300 字符）")
	}

	// 验证选项
	if len(poll.Options) < 2 {
		return fmt.Errorf("投票选项至少需要 2 个")
	}
	if len(poll.Options) > 10 {
		return fmt.Errorf("投票选项最多 10 个")
	}

	for _, opt := range poll.Options {
		if opt.Text == "" {
			return fmt.Errorf("投票选项不能为空")
		}
		if len(opt.Text) > 100 {
			return fmt.Errorf("投票选项过长（最多 100 字符）")
		}
	}

	// 验证测验类型
	if poll.IsQuiz() {
		if poll.CorrectOptionID < 0 || poll.CorrectOptionID >= len(poll.Options) {
			return fmt.Errorf("测验的正确答案索引无效")
		}
	}

	return nil
}
