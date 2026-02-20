package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/features/calculator"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// 正则表达式
var (
	// 符号格式：+100*7.2U 或 -50/2Y
	symbolPattern = regexp.MustCompile(`^([+-])((?:\d+(?:\.\d+)?)(?:[\+\-\*/]\d+(?:\.\d+)?)*)([UY])$`)
	// 中文格式：入100*7.2 或 出50Y
	chinesePattern = regexp.MustCompile(`^(入|出)((?:\d+(?:\.\d+)?)(?:[\+\-\*/]\d+(?:\.\d+)?)*)([UY])?$`)
)

// AccountingServiceImpl 收支记账服务实现
type AccountingServiceImpl struct {
	accountingRepo repository.AccountingRepository
	groupRepo      repository.GroupRepository
	location       *time.Location
}

// NewAccountingService 创建记账服务
func NewAccountingService(accountingRepo repository.AccountingRepository, groupRepo repository.GroupRepository) AccountingService {
	return &AccountingServiceImpl{
		accountingRepo: accountingRepo,
		groupRepo:      groupRepo,
		location:       loadAccountingLocation(),
	}
}

func loadAccountingLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// AddRecord 添加记账记录
func (s *AccountingServiceImpl) AddRecord(ctx context.Context, chatID, userID int64, input string) error {
	// 解析输入
	isIncome, expression, currency, err := s.parseInput(input)
	if err != nil {
		return err
	}

	// 计算表达式
	amount, err := calculator.Calculate(expression)
	if err != nil {
		logger.L().Errorf("Failed to calculate expression %s: %v", expression, err)
		return fmt.Errorf("计算失败: %v", err)
	}
	if amount <= 0 {
		return fmt.Errorf("金额必须大于0")
	}

	// 如果是支出，金额为负数
	if !isIncome {
		amount = -amount
	}

	// 创建记录
	record := &models.AccountingRecord{
		ChatID:       chatID,
		UserID:       userID,
		Amount:       amount,
		Currency:     currency,
		OriginalExpr: expression,
		RecordedAt:   time.Now().In(s.location),
	}

	if err := s.accountingRepo.CreateRecord(ctx, record); err != nil {
		logger.L().Errorf("Failed to create accounting record: %v", err)
		return fmt.Errorf("记录保存失败")
	}

	logger.L().Infof("Accounting record created: chat_id=%d, user_id=%d, amount=%.2f, currency=%s", chatID, userID, amount, currency)
	return nil
}

// parseInput 解析记账输入
func (s *AccountingServiceImpl) parseInput(input string) (isIncome bool, expression string, currency string, err error) {
	input = strings.TrimSpace(input)

	// 尝试符号格式：+100*7.2U 或 -50/2Y
	if matches := symbolPattern.FindStringSubmatch(input); matches != nil {
		sign := matches[1]
		expression = matches[2]
		currencyCode := matches[3]

		isIncome = (sign == "+")
		currency = parseCurrency(currencyCode)
		return
	}

	// 尝试中文格式：入100*7.2 或 出50Y
	if matches := chinesePattern.FindStringSubmatch(input); matches != nil {
		action := matches[1]
		expression = matches[2]
		currencyCode := matches[3]

		isIncome = (action == "入")
		// 如果没有货币后缀，默认为USDT
		if currencyCode == "" {
			currency = models.CurrencyUSD
		} else {
			currency = parseCurrency(currencyCode)
		}
		return
	}

	err = fmt.Errorf("输入格式错误")
	return
}

// parseCurrency 解析货币代码
func parseCurrency(code string) string {
	if code == "U" {
		return models.CurrencyUSD
	}
	return models.CurrencyCNY
}

// QueryRecords 查询并格式化账单
func (s *AccountingServiceImpl) QueryRecords(ctx context.Context, chatID int64) (string, error) {
	now := time.Now().In(s.location)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
	todayEnd := todayStart.Add(24 * time.Hour)
	yesterdayStart := todayStart.Add(-24 * time.Hour)

	// 查询昨日结余（历史累计）
	usdYesterdayBalance, err := s.calculateBalance(ctx, chatID, time.Time{}, yesterdayStart, models.CurrencyUSD)
	if err != nil {
		return "", err
	}

	cnyYesterdayBalance, err := s.calculateBalance(ctx, chatID, time.Time{}, yesterdayStart, models.CurrencyCNY)
	if err != nil {
		return "", err
	}

	// 查询今日明细
	usdTodayRecords, err := s.accountingRepo.GetRecordsByDateRange(ctx, chatID, todayStart, todayEnd, models.CurrencyUSD)
	if err != nil {
		logger.L().Errorf("Failed to query USD records: %v", err)
		return "", fmt.Errorf("查询失败")
	}

	cnyTodayRecords, err := s.accountingRepo.GetRecordsByDateRange(ctx, chatID, todayStart, todayEnd, models.CurrencyCNY)
	if err != nil {
		logger.L().Errorf("Failed to query CNY records: %v", err)
		return "", fmt.Errorf("查询失败")
	}

	// 计算今日总额
	usdTodayTotal := s.sumRecords(usdTodayRecords)
	cnyTodayTotal := s.sumRecords(cnyTodayRecords)

	// 计算总余额
	usdBalance := usdYesterdayBalance + usdTodayTotal
	cnyBalance := cnyYesterdayBalance + cnyTodayTotal

	// 格式化输出
	return s.formatAccountingReport(now, usdYesterdayBalance, usdTodayRecords, usdBalance, cnyYesterdayBalance, cnyTodayRecords, cnyBalance), nil
}

// calculateBalance 计算余额
func (s *AccountingServiceImpl) calculateBalance(ctx context.Context, chatID int64, startTime, endTime time.Time, currency string) (float64, error) {
	records, err := s.accountingRepo.GetRecordsByDateRange(ctx, chatID, startTime, endTime, currency)
	if err != nil {
		return 0, err
	}
	return s.sumRecords(records), nil
}

// sumRecords 汇总记录金额
func (s *AccountingServiceImpl) sumRecords(records []*models.AccountingRecord) float64 {
	var sum float64
	for _, r := range records {
		sum += r.Amount
	}
	return sum
}

// formatAccountingReport 格式化账单报告
func (s *AccountingServiceImpl) formatAccountingReport(
	now time.Time,
	usdYesterdayBalance float64,
	usdTodayRecords []*models.AccountingRecord,
	usdBalance float64,
	cnyYesterdayBalance float64,
	cnyTodayRecords []*models.AccountingRecord,
	cnyBalance float64,
) string {
	var sb strings.Builder

	// 标题
	sb.WriteString(fmt.Sprintf("📊 账单 - %s\n\n", now.Format("2006-01-02")))

	// USDT 部分
	sb.WriteString("💵 USDT\n")
	sb.WriteString(fmt.Sprintf("昨日结余: %s\n", formatAmount(usdYesterdayBalance)))
	if len(usdTodayRecords) > 0 {
		sb.WriteString("今日明细:\n")
		sb.WriteString(s.formatTodayDetailsBlockquote(usdTodayRecords))
		sb.WriteString("\n")
	} else {
		sb.WriteString("今日明细: 无\n")
	}
	sb.WriteString(fmt.Sprintf("总余额: <b>%s</b>\n\n", formatAmount(usdBalance)))

	// CNY 部分
	sb.WriteString("💴 CNY\n")
	sb.WriteString(fmt.Sprintf("昨日结余: %s\n", formatAmount(cnyYesterdayBalance)))
	if len(cnyTodayRecords) > 0 {
		sb.WriteString("今日明细:\n")
		sb.WriteString(s.formatTodayDetailsBlockquote(cnyTodayRecords))
		sb.WriteString("\n")
	} else {
		sb.WriteString("今日明细: 无\n")
	}
	sb.WriteString(fmt.Sprintf("总余额: <b>%s</b>\n", formatAmount(cnyBalance)))

	return sb.String()
}

func (s *AccountingServiceImpl) formatTodayDetailsBlockquote(records []*models.AccountingRecord) string {
	var sb strings.Builder

	sb.WriteString("<blockquote>")
	for _, r := range records {
		sb.WriteString(fmt.Sprintf("%s %s\n", r.RecordedAt.In(s.location).Format("15:04"), formatAmount(r.Amount)))
	}

	return strings.TrimRight(sb.String(), "\n") + "</blockquote>"
}

// formatAmount 格式化金额（整数去掉.0，正数显示+号）
func formatAmount(amount float64) string {
	if amount == float64(int64(amount)) {
		// 整数，去掉.0
		if amount >= 0 {
			return fmt.Sprintf("+%d", int64(amount))
		}
		return fmt.Sprintf("%d", int64(amount))
	}
	// 小数
	if amount >= 0 {
		return fmt.Sprintf("+%.2f", amount)
	}
	return fmt.Sprintf("%.2f", amount)
}

// GetRecentRecordsForDeletion 获取最近2天记录（用于删除界面）
func (s *AccountingServiceImpl) GetRecentRecordsForDeletion(ctx context.Context, chatID int64) ([]*models.AccountingRecord, error) {
	records, err := s.accountingRepo.GetRecentRecords(ctx, chatID, 2)
	if err != nil {
		logger.L().Errorf("Failed to get recent records: %v", err)
		return nil, fmt.Errorf("查询失败")
	}
	return records, nil
}

// DeleteRecord 删除记录
func (s *AccountingServiceImpl) DeleteRecord(ctx context.Context, chatID int64, recordID string) error {
	if err := s.accountingRepo.DeleteRecord(ctx, chatID, recordID); err != nil {
		logger.L().Errorf("Failed to delete record %s for chat %d: %v", recordID, chatID, err)
		return fmt.Errorf("删除失败")
	}
	logger.L().Infof("Accounting record %s deleted for chat %d", recordID, chatID)
	return nil
}

// ClearAllRecords 清空所有记录
func (s *AccountingServiceImpl) ClearAllRecords(ctx context.Context, chatID int64) (int64, error) {
	count, err := s.accountingRepo.DeleteAllByChatID(ctx, chatID)
	if err != nil {
		logger.L().Errorf("Failed to clear all records for chat %d: %v", chatID, err)
		return 0, fmt.Errorf("清空失败")
	}
	logger.L().Infof("Cleared %d accounting records for chat %d", count, chatID)
	return count, nil
}
