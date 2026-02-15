package sifang

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/payment/sifang"
	"go_bot/internal/telegram/features/calculator"
	cryptofeature "go_bot/internal/telegram/features/crypto"
	"go_bot/internal/telegram/features/types"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

var (
	chinaLocation          = mustLoadChinaLocation()
	dateSuffixRegexp       = regexp.MustCompile(`^[0-9\s./\-年月日号]*$`)
	googleCodeSuffixRegexp = regexp.MustCompile(`\s+(\d{6})$`)
	fetchC2COrders         = cryptofeature.FetchC2COrders
	createOrderPrefixes    = []string{"模拟下单", "模拟创建订单"}
)

const (
	SendMoneyConfirmTTL     = 60 * time.Second
	SendMoneyCallbackPrefix = "sifang:sendmoney:"
	sendMoneyActionConfirm  = "confirm"
	sendMoneyActionCancel   = "cancel"
)

type pendingSendMoney struct {
	token      string
	chatID     int64
	userID     int64
	merchantID int64
	amount     float64
	quote      *sendMoneyQuoteSnapshot
	googleCode string
	createdAt  time.Time
}

type sendMoneyQuoteSnapshot struct {
	rate       float64
	usdtAmount float64
}

type sendMoneyQuote struct {
	paymentMethodName string
	orders            []cryptofeature.C2COrder
	serialNum         int
	basePrice         float64
	floatRate         float64
	unitPrice         float64
	usdtAmount        float64
}

func mustLoadChinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// Feature 四方支付功能
type Feature struct {
	paymentService    paymentservice.Service
	userService       service.UserService
	withdrawQuoteRepo repository.WithdrawQuoteRepository
	mu                sync.Mutex
	pending           map[string]*pendingSendMoney
}

// New 创建四方支付功能实例
func New(paymentSvc paymentservice.Service, userSvc service.UserService) *Feature {
	return &Feature{
		paymentService: paymentSvc,
		userService:    userSvc,
		pending:        make(map[string]*pendingSendMoney),
	}
}

// SetWithdrawQuoteRepository 设置下发汇率快照仓储（可选）
func (f *Feature) SetWithdrawQuoteRepository(repo repository.WithdrawQuoteRepository) {
	f.withdrawQuoteRepo = repo
}

// Name 功能名称
func (f *Feature) Name() string {
	return "sifang_payment"
}

// AllowedGroupTiers 仅允许商户群使用四方支付指令
func (f *Feature) AllowedGroupTiers() []models.GroupTier {
	return []models.GroupTier{
		models.GroupTierMerchant,
	}
}

// Enabled 仅在群组启用且服务已配置时生效
func (f *Feature) Enabled(ctx context.Context, group *models.Group) bool {
	return group.Settings.SifangEnabled
}

// Match 支持命令：
//   - 余额
//   - 账单 / 账单10月26（可指定日期）
//   - 下发 [金额 or 表达式] [可选谷歌验证码]
//   - 模拟下单 / 模拟创建订单 [金额 or 表达式] [可选通道代码] [可选订单号]
//   - 下发 [a|z|k|w][序号] [U金额] [可选谷歌验证码]
func (f *Feature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return false
	}

	if _, ok := extractDateSuffix(text, "余额"); ok {
		return true
	}

	if _, ok := extractDateSuffix(text, "账单"); ok {
		return true
	}

	if _, ok := extractDateSuffix(text, "通道账单"); ok {
		return true
	}

	if _, ok := extractDateSuffix(text, "提款明细"); ok {
		return true
	}

	if text == "费率" {
		return true
	}

	if isSendMoneyCommand(text) {
		return true
	}

	if isCreateOrderCommand(text) {
		return true
	}

	return false
}

// Process 执行四方支付查询
func (f *Feature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (*types.Response, bool, error) {
	if f.paymentService == nil {
		return wrapResponse("❌ 未配置四方支付服务，请联系管理员"), true, nil
	}

	if msg.From == nil {
		return nil, false, nil
	}

	merchantID := int64(group.Settings.MerchantID)
	if merchantID == 0 {
		return wrapResponse("ℹ️ 当前群组未绑定商户号，请先使用「绑定 [商户号]」命令"), true, nil
	}

	text := strings.TrimSpace(msg.Text)
	if suffix, ok := extractDateSuffix(text, "余额"); ok {
		respText, handled, err := f.handleBalance(ctx, merchantID, suffix)
		return wrapResponse(respText), handled, err
	}

	if text == "费率" {
		respText, handled, err := f.handleChannelRates(ctx, merchantID)
		return wrapResponse(respText), handled, err
	}

	if _, ok := extractDateSuffix(text, "账单"); ok {
		respText, handled, err := f.handleSummary(ctx, merchantID, text)
		return wrapResponse(respText), handled, err
	}

	if _, ok := extractDateSuffix(text, "通道账单"); ok {
		respText, handled, err := f.handleChannelSummary(ctx, merchantID, text)
		return wrapResponse(respText), handled, err
	}

	if _, ok := extractDateSuffix(text, "提款明细"); ok {
		respText, handled, err := f.handleWithdrawList(ctx, merchantID, text)
		return wrapResponse(respText), handled, err
	}

	if isSendMoneyCommand(text) {
		return f.handleSendMoney(ctx, msg, merchantID, group.Settings.CryptoFloatRate, text)
	}

	if isCreateOrderCommand(text) {
		respText, handled, err := f.handleCreateOrder(ctx, msg, merchantID, text)
		return wrapResponse(respText), handled, err
	}

	return nil, false, nil
}

// Priority 设置为 25，介于商户绑定与行情功能之间
func (f *Feature) Priority() int {
	return 25
}

func (f *Feature) handleBalance(ctx context.Context, merchantID int64, rawSuffix string) (string, bool, error) {
	now := time.Now().In(chinaLocation)
	targetDate, err := parseBalanceDate(rawSuffix, now)
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	historyDays := calculateHistoryDays(targetDate, now)
	nowMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if historyDays > 365 {
		historyDays = 365
		targetDate = nowMidnight.AddDate(0, 0, -historyDays)
	}

	balance, err := f.paymentService.GetBalance(ctx, merchantID, historyDays)
	if err != nil {
		logger.L().Errorf("Sifang balance query failed: merchant_id=%d, history_days=%d, err=%v", merchantID, historyDays, err)
		return fmt.Sprintf("❌ 查询余额失败：%v", err), true, nil
	}
	if balance == nil {
		logger.L().Warnf("Sifang balance query returned empty result: merchant_id=%d, history_days=%d", merchantID, historyDays)
		return "ℹ️ 暂未取得余额数据，请稍后重试", true, nil
	}

	amount := strings.TrimSpace(balance.Balance)
	if historyDays > 0 {
		amount = strings.TrimSpace(balance.HistoryBalance)
	}
	amount = emptyFallback(amount, "未知")

	merchant := balance.MerchantID
	if merchant == "" {
		merchant = strconv.FormatInt(merchantID, 10)
	}

	logger.L().Infof("Sifang balance queried: merchant_id=%s history_days=%d date=%s", merchant, historyDays, targetDate.Format("2006-01-02"))
	return amount, true, nil
}

func (f *Feature) handleSummary(ctx context.Context, merchantID int64, text string) (string, bool, error) {
	dateText := strings.TrimSpace(strings.TrimPrefix(text, "账单"))
	now := time.Now().In(chinaLocation)
	targetDate, err := parseSummaryDate(dateText, now, "账单")
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	message, err := f.buildSummaryMessage(ctx, merchantID, targetDate, now)
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	return message, true, nil
}

// BuildSummaryMessage 构建指定日期的账单消息
func (f *Feature) BuildSummaryMessage(ctx context.Context, merchantID int64, targetDate time.Time) (string, error) {
	now := time.Now().In(chinaLocation)
	return f.buildSummaryMessage(ctx, merchantID, targetDate.In(chinaLocation), now)
}

func (f *Feature) buildSummaryMessage(ctx context.Context, merchantID int64, targetDate, now time.Time) (string, error) {
	targetDate = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())

	summary, err := f.paymentService.GetSummaryByDay(ctx, merchantID, targetDate)
	if err != nil {
		logger.L().Errorf("Sifang summary query failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), err)
		return "", fmt.Errorf("查询账单失败：%w", err)
	}

	if summary == nil {
		return fmt.Sprintf("ℹ️ %s 暂无账单数据", targetDate.Format("2006-01-02")), nil
	}

	if strings.TrimSpace(summary.Date) == "" {
		summary.Date = targetDate.Format("2006-01-02")
	}

	historyDays := calculateHistoryDays(targetDate, now)
	balanceAmount, balanceErr := f.queryBalanceAmount(ctx, merchantID, historyDays)
	withdrawMessage, withdrawErr := f.queryWithdrawMessage(ctx, merchantID, targetDate)

	logger.L().Infof("Sifang summary queried: merchant_id=%d, date=%s", merchantID, summary.Date)
	message := formatSummaryMessage(summary)

	if withdrawErr != nil {
		logger.L().Errorf("Sifang withdraw list in summary failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), withdrawErr)
	} else if withdrawMessage != "" {
		message = fmt.Sprintf("%s\n\n%s", message, withdrawMessage)
	}

	if balanceErr != nil {
		logger.L().Errorf("Sifang balance in summary failed: merchant_id=%d, history_days=%d, err=%v", merchantID, historyDays, balanceErr)
	} else if balanceAmount != "" {
		message = fmt.Sprintf("%s\n\n余额：%s", message, balanceAmount)
	}

	return message, nil
}

func (f *Feature) queryBalanceAmount(ctx context.Context, merchantID int64, historyDays int) (string, error) {
	balance, err := f.paymentService.GetBalance(ctx, merchantID, historyDays)
	if err != nil {
		return "", err
	}
	if balance == nil {
		return "", fmt.Errorf("empty balance response")
	}
	amount := strings.TrimSpace(balance.Balance)
	if historyDays > 0 {
		amount = strings.TrimSpace(balance.HistoryBalance)
	}
	return emptyFallback(amount, "未知"), nil
}

func (f *Feature) queryWithdrawMessage(ctx context.Context, merchantID int64, targetDate time.Time) (string, error) {
	start := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	end := start.Add(24*time.Hour - time.Second)

	list, err := f.paymentService.GetWithdrawList(ctx, merchantID, start, end, 1, 100)
	if err != nil {
		return "", err
	}

	filtered := filterSuccessfulWithdrawList(list)
	quoteLookup := f.loadWithdrawQuoteLookup(ctx, merchantID, start, start.Add(24*time.Hour))
	return formatWithdrawListMessageWithQuotes(targetDate.Format("2006-01-02"), filtered, quoteLookup), nil
}

func filterSuccessfulWithdrawList(list *paymentservice.WithdrawList) *paymentservice.WithdrawList {
	if list == nil {
		return &paymentservice.WithdrawList{Items: []*paymentservice.Withdraw{}}
	}

	filteredItems := make([]*paymentservice.Withdraw, 0, len(list.Items))
	for _, item := range list.Items {
		if isSuccessfulWithdraw(item) {
			filteredItems = append(filteredItems, item)
		}
	}

	copy := *list
	copy.Total = len(filteredItems)
	copy.Items = filteredItems
	return &copy
}

func isSuccessfulWithdraw(item *paymentservice.Withdraw) bool {
	if item == nil {
		return false
	}

	status := strings.ToLower(strings.TrimSpace(item.Status))
	switch status {
	case "1", "paid", "success", "succeed", "succeeded", "completed", "complete", "done", "已支付", "支付成功", "成功":
		return true
	default:
		return strings.TrimSpace(item.PaidAt) != ""
	}
}

func parseSummaryDate(raw string, now time.Time, usage string) (time.Time, error) {
	usage = strings.TrimSpace(usage)
	if usage == "" {
		usage = "账单"
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	}

	normalized := strings.ToLower(raw)
	normalized = strings.ReplaceAll(normalized, "日", "")
	normalized = strings.ReplaceAll(normalized, "号", "")
	normalized = strings.ReplaceAll(normalized, "年", "-")
	normalized = strings.ReplaceAll(normalized, "月", "-")
	normalized = strings.ReplaceAll(normalized, "/", "-")
	normalized = strings.ReplaceAll(normalized, ".", "-")
	normalized = strings.Trim(normalized, "- ")
	if normalized == "" {
		return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
	}

	parts := strings.Split(normalized, "-")
	var (
		year  int
		month int
		day   int
		err   error
	)

	switch len(parts) {
	case 3:
		year, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
		month, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
		day, err = strconv.Atoi(parts[2])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
	case 2:
		year = now.Year()
		month, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
		day, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
	default:
		return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
	}

	candidate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location())
	if candidate.Month() != time.Month(month) || candidate.Day() != day || candidate.Year() != year {
		return time.Time{}, fmt.Errorf("日期不存在，请检查月份和日期")
	}

	if len(parts) == 2 && candidate.After(now) {
		candidate = candidate.AddDate(-1, 0, 0)
	}

	return candidate, nil
}

// ParseSummaryDate 暴露给其他功能复用的日期解析
func ParseSummaryDate(raw string, now time.Time, usage string) (time.Time, error) {
	return parseSummaryDate(raw, now, usage)
}

func parseBalanceDate(raw string, now time.Time) (time.Time, error) {
	return parseSummaryDate(raw, now, "余额")
}

func calculateHistoryDays(target, now time.Time) int {
	targetMidnight := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, target.Location())
	nowMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if targetMidnight.After(nowMidnight) {
		return 0
	}

	days := int(nowMidnight.Sub(targetMidnight).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days
}

func formatSummaryMessage(summary *paymentservice.SummaryByDay) string {
	var sb strings.Builder

	date := strings.TrimSpace(summary.Date)
	if date == "" {
		date = "-"
	}
	sb.WriteString(fmt.Sprintf("📑 账单 - %s\n", html.EscapeString(date)))

	if value := strings.TrimSpace(summary.TotalAmount); value != "" {
		sb.WriteString(fmt.Sprintf("跑量：%s\n", html.EscapeString(value)))
	}
	if combinedIncome := combineAmounts(summary.MerchantIncome, summary.AgentIncome); combinedIncome != "" {
		sb.WriteString(fmt.Sprintf("成交：%s\n", html.EscapeString(combinedIncome)))
	}
	if value := strings.TrimSpace(summary.OrderCount); value != "" {
		sb.WriteString(fmt.Sprintf("笔数：%s\n", html.EscapeString(value)))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (f *Feature) handleChannelSummary(ctx context.Context, merchantID int64, text string) (string, bool, error) {
	dateText := strings.TrimSpace(strings.TrimPrefix(text, "通道账单"))
	now := time.Now().In(chinaLocation)
	targetDate, err := parseSummaryDate(dateText, now, "通道账单")
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	items, err := f.paymentService.GetSummaryByDayByChannel(ctx, merchantID, targetDate)
	if err != nil {
		logger.L().Errorf("Sifang channel summary query failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), err)
		return fmt.Sprintf("❌ 查询通道账单失败：%v", err), true, nil
	}

	if len(items) == 0 {
		return fmt.Sprintf("ℹ️ %s 暂无通道账单数据", targetDate.Format("2006-01-02")), true, nil
	}

	logger.L().Infof("Sifang channel summary queried: merchant_id=%d, date=%s, channels=%d", merchantID, targetDate.Format("2006-01-02"), len(items))

	message := formatChannelSummaryMessage(targetDate.Format("2006-01-02"), items)

	historyDays := calculateHistoryDays(targetDate, now)
	balanceAmount, balanceErr := f.queryBalanceAmount(ctx, merchantID, historyDays)
	withdrawMessage, withdrawErr := f.queryWithdrawMessage(ctx, merchantID, targetDate)

	if withdrawErr != nil {
		logger.L().Errorf("Sifang withdraw list in channel summary failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), withdrawErr)
	} else if withdrawMessage != "" {
		message = fmt.Sprintf("%s\n\n%s", message, withdrawMessage)
	}

	if balanceErr != nil {
		logger.L().Errorf("Sifang balance in channel summary failed: merchant_id=%d, history_days=%d, err=%v", merchantID, historyDays, balanceErr)
	} else if balanceAmount != "" {
		message = fmt.Sprintf("%s\n\n余额：%s", message, balanceAmount)
	}

	return message, true, nil
}

func formatChannelSummaryMessage(date string, items []*paymentservice.SummaryByDayChannel) string {
	if len(items) == 0 {
		return fmt.Sprintf("ℹ️ %s 暂无通道账单数据", html.EscapeString(date))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📑 通道账单 - %s\n", html.EscapeString(date)))

	for _, item := range items {
		name := strings.TrimSpace(item.ChannelName)
		code := strings.TrimSpace(item.ChannelCode)

		sb.WriteString("\n")
		switch {
		case name != "" && code != "":
			sb.WriteString(fmt.Sprintf("%s：<code>%s</code>\n", html.EscapeString(name), html.EscapeString(code)))
		case name != "":
			sb.WriteString(fmt.Sprintf("%s\n", html.EscapeString(name)))
		case code != "":
			sb.WriteString(fmt.Sprintf("<code>%s</code>\n", html.EscapeString(code)))
		default:
			sb.WriteString("-\n")
		}

		volume := strings.TrimSpace(item.TotalAmount)
		if volume == "" {
			volume = "0"
		}
		sb.WriteString(fmt.Sprintf("跑量：%s\n", html.EscapeString(volume)))

		combined := combineAmounts(item.MerchantIncome, item.AgentIncome)
		if combined == "" {
			combined = "0"
		}
		sb.WriteString(fmt.Sprintf("成交：%s\n", html.EscapeString(combined)))

		count := strings.TrimSpace(item.OrderCount)
		if count == "" {
			count = "0"
		}
		sb.WriteString(fmt.Sprintf("笔数：%s\n", html.EscapeString(count)))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (f *Feature) handleWithdrawList(ctx context.Context, merchantID int64, text string) (string, bool, error) {
	dateText := strings.TrimSpace(strings.TrimPrefix(text, "提款明细"))
	now := time.Now().In(chinaLocation)
	targetDate, err := parseSummaryDate(dateText, now, "提款明细")
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	start := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	end := start.Add(24*time.Hour - time.Second)

	list, err := f.paymentService.GetWithdrawList(ctx, merchantID, start, end, 1, 10)
	if err != nil {
		logger.L().Errorf("Sifang withdraw list query failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), err)
		return fmt.Sprintf("❌ 查询提款明细失败：%v", err), true, nil
	}

	filtered := filterSuccessfulWithdrawList(list)
	quoteLookup := f.loadWithdrawQuoteLookup(ctx, merchantID, start, start.Add(24*time.Hour))
	message := formatWithdrawListMessageWithQuotes(targetDate.Format("2006-01-02"), filtered, quoteLookup)
	itemCount := 0
	if filtered != nil {
		itemCount = len(filtered.Items)
	}
	logger.L().Infof("Sifang withdraw list queried: merchant_id=%d, date=%s, count=%d", merchantID, targetDate.Format("2006-01-02"), itemCount)
	return message, true, nil
}

func (f *Feature) loadWithdrawQuoteLookup(ctx context.Context, merchantID int64, start, end time.Time) map[string]*models.WithdrawQuoteRecord {
	if f.withdrawQuoteRepo == nil {
		return nil
	}

	records, err := f.withdrawQuoteRepo.ListByMerchantAndDateRange(ctx, merchantID, start, end)
	if err != nil {
		logger.L().Errorf("Sifang withdraw quote query failed: merchant_id=%d, err=%v", merchantID, err)
		return nil
	}

	return buildWithdrawQuoteLookup(records)
}

func formatWithdrawListMessage(date string, list *paymentservice.WithdrawList) string {
	return formatWithdrawListMessageWithQuotes(date, list, nil)
}

func formatWithdrawListMessageWithQuotes(date string, list *paymentservice.WithdrawList, quoteLookup map[string]*models.WithdrawQuoteRecord) string {
	var sb strings.Builder

	totalAmount := 0.0
	itemCount := 0
	items := []*paymentservice.Withdraw{}
	if list != nil {
		items = list.Items
	}
	for _, item := range items {
		if amount, ok := parseAmountToFloat(item.Amount); ok {
			totalAmount += amount
		}
		itemCount++
	}

	title := "💸 提款明细"

	if itemCount == 0 {
		return fmt.Sprintf("%s\n暂无提款记录", title)
	}

	sb.WriteString(fmt.Sprintf("%s（总计 %s｜%d 笔）\n", title, html.EscapeString(formatFloat(totalAmount)), itemCount))
	sb.WriteString("<blockquote>")

	for _, item := range items {
		created := strings.TrimSpace(item.CreatedAt)
		timePart := extractTime(created)
		if timePart == "" {
			timePart = "--:--:--"
		}

		amount := strings.TrimSpace(item.Amount)
		if amount == "" {
			amount = "0"
		}

		quoteText := buildWithdrawQuoteText(item, quoteLookup)
		if quoteText == "" {
			sb.WriteString(fmt.Sprintf("%s      %s\n", html.EscapeString(timePart), html.EscapeString(amount)))
		} else {
			sb.WriteString(fmt.Sprintf("%s      %s      %s\n",
				html.EscapeString(timePart),
				html.EscapeString(amount),
				html.EscapeString(quoteText),
			))
		}
	}

	return strings.TrimRight(sb.String(), "\n") + "</blockquote>"
}

func buildWithdrawQuoteLookup(records []*models.WithdrawQuoteRecord) map[string]*models.WithdrawQuoteRecord {
	if len(records) == 0 {
		return nil
	}

	lookup := make(map[string]*models.WithdrawQuoteRecord, len(records)*2)
	for _, record := range records {
		if record == nil {
			continue
		}

		if key := buildWithdrawLookupKey("withdraw_no", record.WithdrawNo); key != "" {
			lookup[key] = record
		}
		if key := buildWithdrawLookupKey("order_no", record.OrderNo); key != "" {
			if _, exists := lookup[key]; !exists {
				lookup[key] = record
			}
		}
	}

	if len(lookup) == 0 {
		return nil
	}
	return lookup
}

func buildWithdrawQuoteText(item *paymentservice.Withdraw, lookup map[string]*models.WithdrawQuoteRecord) string {
	record := findWithdrawQuoteRecord(item, lookup)
	if record == nil || record.Rate <= 0 || record.USDTAmount <= 0 {
		return ""
	}
	return fmt.Sprintf("%s ✖️ %s U", formatFloat(record.Rate), formatFloat(record.USDTAmount))
}

func findWithdrawQuoteRecord(item *paymentservice.Withdraw, lookup map[string]*models.WithdrawQuoteRecord) *models.WithdrawQuoteRecord {
	if item == nil || len(lookup) == 0 {
		return nil
	}

	if key := buildWithdrawLookupKey("withdraw_no", item.WithdrawNo); key != "" {
		if record, ok := lookup[key]; ok {
			return record
		}
	}
	if key := buildWithdrawLookupKey("order_no", item.OrderNo); key != "" {
		if record, ok := lookup[key]; ok {
			return record
		}
	}
	return nil
}

func buildWithdrawLookupKey(prefix, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return prefix + ":" + trimmed
}

type createOrderCommand struct {
	amount          float64
	channelCode     string
	merchantOrderNo string
}

func (f *Feature) handleCreateOrder(ctx context.Context, msg *botModels.Message, merchantID int64, text string) (string, bool, error) {
	if f.userService == nil {
		logger.L().Error("Sifang create order: user service is nil")
		return "❌ 未配置管理员校验服务，请联系管理员", true, nil
	}

	isAdmin, err := f.userService.CheckAdminPermission(ctx, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Sifang create order admin check failed: user_id=%d, err=%v", msg.From.ID, err)
		return "❌ 权限检查失败，请稍后重试", true, nil
	}
	if !isAdmin {
		logger.L().Warnf("Sifang create order unauthorized: user_id=%d, chat_id=%d", msg.From.ID, msg.Chat.ID)
		return "❌ 仅管理员可以模拟下单", true, nil
	}

	cmd, err := parseCreateOrderCommand(text)
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	req := paymentservice.CreateOrderRequest{
		Amount:          cmd.amount,
		ChannelCode:     cmd.channelCode,
		MerchantOrderNo: cmd.merchantOrderNo,
	}

	result, err := f.paymentService.CreateOrder(ctx, merchantID, req)
	if err != nil {
		logger.L().Errorf("Sifang create order failed: merchant_id=%d, user_id=%d, amount=%.2f, err=%v", merchantID, msg.From.ID, cmd.amount, err)
		return fmt.Sprintf("❌ 模拟下单失败：%v", err), true, nil
	}
	if result == nil {
		return "❌ 模拟下单失败：返回数据为空", true, nil
	}

	logger.L().Infof("Sifang create order success: merchant_id=%d, user_id=%d, amount=%.2f, order_no=%s, channel=%s",
		merchantID, msg.From.ID, cmd.amount, result.MerchantOrderNo, result.ChannelCode)

	return formatCreateOrderMessage(merchantID, cmd.amount, result), true, nil
}

func parseCreateOrderCommand(text string) (*createOrderCommand, error) {
	payload, ok := trimCreateOrderPrefix(text)
	if !ok {
		return nil, fmt.Errorf("请使用：模拟下单 <金额> [通道代码] [订单号]")
	}
	if payload == "" {
		return nil, fmt.Errorf("请使用：模拟下单 <金额> [通道代码] [订单号]")
	}

	fields := strings.Fields(payload)
	if len(fields) == 0 {
		return nil, fmt.Errorf("请使用：模拟下单 <金额> [通道代码] [订单号]")
	}
	if len(fields) > 3 {
		return nil, fmt.Errorf("参数过多，请使用：模拟下单 <金额> [通道代码] [订单号]")
	}

	amount, err := parseSendMoneyAmount(fields[0])
	if err != nil {
		return nil, err
	}

	cmd := &createOrderCommand{amount: amount}
	if len(fields) >= 2 {
		cmd.channelCode = strings.TrimSpace(fields[1])
	}
	if len(fields) >= 3 {
		cmd.merchantOrderNo = strings.TrimSpace(fields[2])
	}

	return cmd, nil
}

func formatCreateOrderMessage(merchantID int64, requestAmount float64, result *paymentservice.CreateOrderResult) string {
	merchantText := strconv.FormatInt(merchantID, 10)
	if id := strings.TrimSpace(result.MerchantID); id != "" {
		merchantText = id
	}

	orderNo := strings.TrimSpace(result.MerchantOrderNo)
	if orderNo == "" {
		orderNo = "-"
	}

	amountText := formatFloat(requestAmount)
	if amount, ok := parseAmountToFloat(strings.TrimSpace(result.Amount)); ok {
		amountText = formatFloat(amount)
	}

	channel := strings.TrimSpace(result.ChannelCode)
	if channel == "" {
		channel = "-"
	}

	var sb strings.Builder
	sb.WriteString("🧪 模拟下单成功")
	sb.WriteString(fmt.Sprintf("\n商户：%s", html.EscapeString(merchantText)))
	sb.WriteString(fmt.Sprintf("\n订单号：<code>%s</code>", html.EscapeString(orderNo)))
	sb.WriteString(fmt.Sprintf("\n金额：%s", html.EscapeString(amountText)))
	sb.WriteString(fmt.Sprintf("\n通道：<code>%s</code>", html.EscapeString(channel)))

	if payURL := strings.TrimSpace(result.PaymentURL); payURL != "" {
		sb.WriteString(fmt.Sprintf("\n支付链接：%s", html.EscapeString(payURL)))
	}

	if payment := strings.TrimSpace(result.Payment); payment != "" {
		sb.WriteString(fmt.Sprintf("\n支付参数：<code>%s</code>", html.EscapeString(payment)))
	}

	if status := strings.TrimSpace(result.Status); status != "" {
		sb.WriteString(fmt.Sprintf("\n状态：%s", html.EscapeString(status)))
	}

	return sb.String()
}

func (f *Feature) handleSendMoney(ctx context.Context, msg *botModels.Message, merchantID int64, floatRate float64, text string) (*types.Response, bool, error) {
	if f.userService == nil {
		logger.L().Error("Sifang send money: user service is nil")
		return wrapResponse("❌ 未配置管理员校验服务，请联系管理员"), true, nil
	}

	isAdmin, err := f.userService.CheckAdminPermission(ctx, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Sifang send money admin check failed: user_id=%d, err=%v", msg.From.ID, err)
		return wrapResponse("❌ 权限检查失败，请稍后重试"), true, nil
	}
	if !isAdmin {
		logger.L().Warnf("Sifang send money unauthorized: user_id=%d, chat_id=%d", msg.From.ID, msg.Chat.ID)
		return wrapResponse("❌ 仅管理员可以下发"), true, nil
	}

	payload := strings.TrimSpace(strings.TrimPrefix(text, "下发"))
	amount, googleCode, quote, parseErr := f.resolveSendMoneyPayload(ctx, payload, floatRate)
	if parseErr != nil {
		return wrapResponse(fmt.Sprintf("❌ %v", parseErr)), true, nil
	}

	pending, err := f.createPendingSend(msg.Chat.ID, msg.From.ID, merchantID, amount, googleCode)
	if err != nil {
		logger.L().Errorf("Sifang create pending send failed: chat_id=%d, user_id=%d, err=%v", msg.Chat.ID, msg.From.ID, err)
		return wrapResponse("❌ 创建下发确认状态失败，请稍后重试"), true, nil
	}
	pending.quote = snapshotSendMoneyQuote(quote)

	message := buildSendMoneyConfirmationMessage(merchantID, amount, quote)
	if googleCode != "" {
		message += "\n🔐 将附带当前谷歌验证码"
	}

	markup := buildSendMoneyKeyboard(pending.token)

	logger.L().Infof("Sifang send money pending confirmation: merchant_id=%d, user_id=%d, amount=%.2f, token=%s", merchantID, msg.From.ID, amount, pending.token)

	return &types.Response{
		Text:        message,
		ReplyMarkup: markup,
	}, true, nil
}

func snapshotSendMoneyQuote(quote *sendMoneyQuote) *sendMoneyQuoteSnapshot {
	if quote == nil {
		return nil
	}
	rate := roundToTwoDecimals(quote.unitPrice)
	usdtAmount := roundToTwoDecimals(quote.usdtAmount)
	if rate <= 0 || usdtAmount <= 0 {
		return nil
	}
	return &sendMoneyQuoteSnapshot{
		rate:       rate,
		usdtAmount: usdtAmount,
	}
}

func (f *Feature) resolveSendMoneyPayload(ctx context.Context, raw string, floatRate float64) (float64, string, *sendMoneyQuote, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "", nil, fmt.Errorf("下发金额不能为空")
	}

	payload, googleCode := splitSendMoneyGoogleCode(raw)
	if payload == "" {
		return 0, "", nil, fmt.Errorf("下发金额不能为空")
	}

	if cmdInfo, err := cryptofeature.ParseCommand(payload); err == nil {
		if !cmdInfo.HasAmount {
			return 0, "", nil, fmt.Errorf("下发行情指令缺少U金额，示例：下发 z3 100")
		}

		orders, fetchErr := fetchC2COrders(ctx, cmdInfo.PaymentMethod)
		if fetchErr != nil {
			logger.L().Errorf("Sifang send money quote fetch failed: payment_method=%s, err=%v", cmdInfo.PaymentMethod, fetchErr)
			return 0, "", nil, fmt.Errorf("获取报价失败，请稍后重试")
		}

		if cmdInfo.SerialNum > len(orders) {
			return 0, "", nil, fmt.Errorf("商家序号超出范围（最多 %d 个）", len(orders))
		}

		selected := orders[cmdInfo.SerialNum-1]
		basePrice, parseErr := strconv.ParseFloat(strings.TrimSpace(selected.Price), 64)
		if parseErr != nil {
			logger.L().Errorf("Sifang send money quote price parse failed: serial=%d, price=%s, err=%v", cmdInfo.SerialNum, selected.Price, parseErr)
			return 0, "", nil, fmt.Errorf("报价解析失败")
		}

		unitPrice := basePrice + floatRate
		amount := roundToTwoDecimals(unitPrice * cmdInfo.Amount)
		if math.IsNaN(amount) || math.IsInf(amount, 0) {
			return 0, "", nil, fmt.Errorf("金额计算结果异常")
		}
		if amount <= 0 {
			return 0, "", nil, fmt.Errorf("下发金额必须大于 0")
		}

		maxDisplay := 10
		if len(orders) < maxDisplay {
			maxDisplay = len(orders)
		}
		displayOrders := append([]cryptofeature.C2COrder{}, orders[:maxDisplay]...)

		quote := &sendMoneyQuote{
			paymentMethodName: cmdInfo.PaymentMethodName,
			orders:            displayOrders,
			serialNum:         cmdInfo.SerialNum,
			basePrice:         basePrice,
			floatRate:         floatRate,
			unitPrice:         unitPrice,
			usdtAmount:        cmdInfo.Amount,
		}

		return amount, googleCode, quote, nil
	}

	amount, parseErr := parseSendMoneyAmount(payload)
	if parseErr != nil {
		return 0, "", nil, parseErr
	}

	return amount, googleCode, nil, nil
}

func buildSendMoneyConfirmationMessage(merchantID int64, amount float64, quote *sendMoneyQuote) string {
	merchantText := strconv.FormatInt(merchantID, 10)
	if quote == nil {
		return fmt.Sprintf("是否确认下发 %s 元 | %s", html.EscapeString(formatFloat(amount)), html.EscapeString(merchantText))
	}

	var response strings.Builder
	response.WriteString("<b>OTC商家实时价格</b>\n\n")
	response.WriteString(fmt.Sprintf("信息来源: 欧易 <b>%s</b>\n", html.EscapeString(quote.paymentMethodName)))
	response.WriteString("\n")

	for i, order := range quote.orders {
		price, err := strconv.ParseFloat(strings.TrimSpace(order.Price), 64)
		if err != nil {
			price = 0
		}
		name := strings.TrimSpace(order.NickName)
		if name == "" {
			name = "-"
		}

		if i == quote.serialNum-1 {
			if quote.floatRate > 0 {
				response.WriteString(fmt.Sprintf("✅<b>%.2f        %s</b>___➕<b>%.2f</b>🟰<code>%.2f</code>⬅️\n",
					price, html.EscapeString(name), quote.floatRate, quote.unitPrice))
			} else {
				response.WriteString(fmt.Sprintf("✅<b>%.2f        %s</b> 🟰 <code>%.2f</code>⬅️\n",
					price, html.EscapeString(name), quote.unitPrice))
			}
		} else {
			response.WriteString(fmt.Sprintf("     <code>%.2f   %s</code>\n", price, html.EscapeString(name)))
		}
	}

	response.WriteString(fmt.Sprintf("\n<code>%.2f</code> ✖️ <code>%s</code> <b>U</b> 🟰 <code>%.2f</code> <b>¥</b>\n",
		quote.unitPrice, html.EscapeString(formatFloat(quote.usdtAmount)), amount))
	response.WriteString(fmt.Sprintf("是否确认下发 %s 元 | %s",
		html.EscapeString(formatFloat(amount)), html.EscapeString(merchantText)))

	return response.String()
}

func splitSendMoneyGoogleCode(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	googleCode := ""
	if matches := googleCodeSuffixRegexp.FindStringSubmatch(raw); len(matches) == 2 {
		googleCode = matches[1]
		raw = strings.TrimSpace(raw[:len(raw)-len(matches[0])])
	}

	return raw, googleCode
}

func parseSendMoneyPayload(raw string) (float64, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "", fmt.Errorf("下发金额不能为空")
	}

	raw, googleCode := splitSendMoneyGoogleCode(raw)
	if raw == "" {
		return 0, "", fmt.Errorf("下发金额不能为空")
	}

	amount, err := parseSendMoneyAmount(raw)
	if err != nil {
		return 0, "", err
	}

	return amount, googleCode, nil
}

func parseSendMoneyAmount(raw string) (float64, error) {
	var (
		amount float64
		err    error
	)

	if calculator.IsMathExpression(raw) {
		amount, err = calculator.Calculate(raw)
		if err != nil {
			return 0, fmt.Errorf("金额计算失败：%v", err)
		}
	} else {
		amount, err = strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
		if err != nil {
			return 0, fmt.Errorf("金额格式错误")
		}
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, fmt.Errorf("金额计算结果异常")
	}

	amount = roundToTwoDecimals(amount)
	if amount <= 0 {
		return 0, fmt.Errorf("下发金额必须大于 0")
	}

	return amount, nil
}

func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

func formatSendMoneyMessage(merchantID int64, requestAmount float64, result *paymentservice.SendMoneyResult) string {
	amountText := formatFloat(requestAmount)
	if result != nil && result.Withdraw != nil {
		if amt := strings.TrimSpace(result.Withdraw.Amount); amt != "" {
			if numeric, ok := parseAmountToFloat(amt); ok && numeric > 0 {
				amountText = formatFloat(numeric)
			}
		}
	}

	merchantText := strconv.FormatInt(merchantID, 10)
	if result != nil {
		if id := strings.TrimSpace(result.MerchantID); id != "" {
			merchantText = id
		}
	}

	return fmt.Sprintf("已成功下发 <code>%s</code> 元给商户 <code>%s</code>",
		html.EscapeString(amountText),
		html.EscapeString(merchantText),
	)
}

func combineAmounts(merchant, agent string) string {
	merchant = strings.TrimSpace(merchant)
	agent = strings.TrimSpace(agent)

	if merchant == "" && agent == "" {
		return ""
	}

	merchantVal, ok1 := parseAmountToFloat(merchant)
	agentVal, ok2 := parseAmountToFloat(agent)

	if ok1 || ok2 {
		total := 0.0
		if ok1 {
			total += merchantVal
		}
		if ok2 {
			total += agentVal
		}
		return formatFloat(total)
	}

	if agent == "" {
		return merchant
	}
	if merchant == "" {
		return agent
	}
	return merchant + agent
}

func parseAmountToFloat(input string) (float64, bool) {
	if input == "" {
		return 0, false
	}
	cleaned := strings.ReplaceAll(input, ",", "")
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func formatFloat(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

func extractTime(datetime string) string {
	datetime = strings.TrimSpace(datetime)
	if datetime == "" {
		return ""
	}

	if len(datetime) >= 8 {
		idx := strings.LastIndex(datetime, " ")
		if idx >= 0 && idx+1 < len(datetime) {
			timePart := datetime[idx+1:]
			if len(timePart) == 8 {
				return timePart
			}
		}

		if len(datetime) >= 8 {
			candidate := datetime[len(datetime)-8:]
			if strings.Count(candidate, ":") == 2 {
				return candidate
			}
		}
	}

	return ""
}

func extractDateSuffix(text, prefix string) (string, bool) {
	if !strings.HasPrefix(text, prefix) {
		return "", false
	}

	suffix := text[len(prefix):]
	if !isValidDateSuffix(suffix) {
		return "", false
	}
	return suffix, true
}

func isValidDateSuffix(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return true
	}
	return dateSuffixRegexp.MatchString(trimmed)
}

func isSendMoneyCommand(text string) bool {
	if !strings.HasPrefix(text, "下发") {
		return false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(text, "下发"))
	return payload != ""
}

func isCreateOrderCommand(text string) bool {
	payload, ok := trimCreateOrderPrefix(text)
	if !ok {
		return false
	}
	return payload != ""
}

func trimCreateOrderPrefix(text string) (string, bool) {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return "", false
	}
	for _, prefix := range createOrderPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(normalized, prefix)), true
		}
	}
	return "", false
}

func (f *Feature) createPendingSend(chatID, userID, merchantID int64, amount float64, googleCode string) (*pendingSendMoney, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	pending := &pendingSendMoney{
		token:      token,
		chatID:     chatID,
		userID:     userID,
		merchantID: merchantID,
		amount:     amount,
		googleCode: googleCode,
		createdAt:  time.Now(),
	}

	f.mu.Lock()
	f.cleanupExpiredLocked()
	for {
		if _, exists := f.pending[pending.token]; !exists {
			f.pending[pending.token] = pending
			break
		}
		token, err = generateToken()
		if err != nil {
			f.mu.Unlock()
			return nil, err
		}
		pending.token = token
	}
	f.mu.Unlock()

	return pending, nil
}

func (f *Feature) getPendingByToken(token string) (*pendingSendMoney, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupExpiredLocked()
	pending, ok := f.pending[token]
	return pending, ok
}

func (f *Feature) deletePending(token string) {
	f.mu.Lock()
	delete(f.pending, token)
	f.mu.Unlock()
}

func (f *Feature) cleanupExpiredLocked() {
	if len(f.pending) == 0 {
		return
	}
	now := time.Now()
	for token, pending := range f.pending {
		if now.Sub(pending.createdAt) > SendMoneyConfirmTTL {
			delete(f.pending, token)
		}
	}
}

// ExpirePending 在确认超时后删除待处理请求
func (f *Feature) ExpirePending(token string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	pending, ok := f.pending[token]
	if !ok {
		return false
	}

	if time.Since(pending.createdAt) < SendMoneyConfirmTTL {
		return false
	}

	delete(f.pending, token)
	logger.L().Infof("Sifang send money pending expired: token=%s user_id=%d merchant_id=%d amount=%.2f", token, pending.userID, pending.merchantID, pending.amount)
	return true
}

func generateToken() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func buildSendMoneyKeyboard(token string) *botModels.InlineKeyboardMarkup {
	confirmData := sendMoneyCallbackData(sendMoneyActionConfirm, token)
	cancelData := sendMoneyCallbackData(sendMoneyActionCancel, token)
	keyboard := [][]botModels.InlineKeyboardButton{
		{
			{
				Text:         "❌取消",
				CallbackData: cancelData,
			},
			{
				Text:         "✅确认",
				CallbackData: confirmData,
			},
		},
	}
	return &botModels.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func sendMoneyCallbackData(action, token string) string {
	return SendMoneyCallbackPrefix + action + ":" + token
}

// SendMoneyCallbackResult 表示处理回调后的结果
type SendMoneyCallbackResult struct {
	ShouldEdit   bool
	Text         string
	Markup       botModels.ReplyMarkup
	Answer       string
	ShowAlert    bool
	FollowupText string
}

// HandleSendMoneyCallback 处理确认/取消回调
func (f *Feature) HandleSendMoneyCallback(ctx context.Context, query *botModels.CallbackQuery, action, token string) (*SendMoneyCallbackResult, error) {
	result := &SendMoneyCallbackResult{
		Markup: nil,
	}

	pending, ok := f.getPendingByToken(token)
	if !ok {
		result.ShouldEdit = true
		result.Text = "下发请求已过期"
		result.Answer = "操作已过期"
		return result, nil
	}

	if query.From.ID != pending.userID {
		result.ShouldEdit = false
		result.Answer = "仅原管理员可以操作此下发"
		result.ShowAlert = true
		return result, nil
	}

	switch action {
	case sendMoneyActionCancel:
		f.deletePending(token)
		result.ShouldEdit = true
		merchantText := strconv.FormatInt(pending.merchantID, 10)
		result.Text = fmt.Sprintf("已取消下发 <code>%s</code> 元给商户 <code>%s</code>",
			html.EscapeString(formatFloat(pending.amount)),
			html.EscapeString(merchantText),
		)
		result.Answer = "已取消"
		return result, nil
	case sendMoneyActionConfirm:
		f.deletePending(token)
		opts := paymentservice.SendMoneyOptions{GoogleCode: pending.googleCode}
		sendResult, err := f.paymentService.SendMoney(ctx, pending.merchantID, pending.amount, opts)
		if err != nil {
			logger.L().Errorf("Sifang send money (callback) failed: merchant_id=%d, user_id=%d, amount=%.2f, err=%v", pending.merchantID, pending.userID, pending.amount, err)
			var apiErr *sifang.APIError
			if errors.As(err, &apiErr) {
				logger.L().Errorf("Sifang send money API error detail: code=%d message=%s", apiErr.Code, apiErr.Message)
				result.Text = fmt.Sprintf("下发失败：%s", html.EscapeString(apiErr.Message))
			} else {
				result.Text = fmt.Sprintf("下发失败：%s", html.EscapeString(err.Error()))
			}
			result.ShouldEdit = true
			result.Answer = "下发失败"
			return result, nil
		}

		message := formatSendMoneyMessage(pending.merchantID, pending.amount, sendResult)
		f.persistSendMoneyQuote(ctx, pending, sendResult)
		if sendResult != nil && sendResult.Withdraw != nil {
			logger.L().Infof("Sifang send money response detail: merchant_id=%d, withdraw_no=%s, response_amount=%s, status=%s",
				pending.merchantID,
				strings.TrimSpace(sendResult.Withdraw.WithdrawNo),
				strings.TrimSpace(sendResult.Withdraw.Amount),
				strings.TrimSpace(sendResult.Withdraw.Status),
			)
		}
		logger.L().Infof("Sifang send money success: merchant_id=%d, user_id=%d, amount=%.2f", pending.merchantID, pending.userID, pending.amount)

		result.ShouldEdit = true
		result.Text = message
		result.Answer = "下发成功"
		summaryMessage, _, summaryErr := f.handleSummary(ctx, pending.merchantID, "账单")
		if summaryErr != nil {
			logger.L().Errorf("Sifang auto summary after send money failed: merchant_id=%d, err=%v", pending.merchantID, summaryErr)
		} else if strings.TrimSpace(summaryMessage) != "" {
			result.FollowupText = summaryMessage
		}
		return result, nil
	default:
		result.ShouldEdit = false
		result.Answer = "未知操作"
		result.ShowAlert = true
		return result, nil
	}
}

func (f *Feature) persistSendMoneyQuote(ctx context.Context, pending *pendingSendMoney, sendResult *paymentservice.SendMoneyResult) {
	if f.withdrawQuoteRepo == nil || pending == nil || pending.quote == nil {
		return
	}

	record := &models.WithdrawQuoteRecord{
		MerchantID: pending.merchantID,
		ChatID:     pending.chatID,
		UserID:     pending.userID,
		Amount:     pending.amount,
		Rate:       pending.quote.rate,
		USDTAmount: pending.quote.usdtAmount,
		CreatedAt:  time.Now(),
	}

	if sendResult != nil && sendResult.Withdraw != nil {
		record.WithdrawNo = strings.TrimSpace(sendResult.Withdraw.WithdrawNo)
		record.OrderNo = strings.TrimSpace(sendResult.Withdraw.OrderNo)
		if amount, ok := parseAmountToFloat(strings.TrimSpace(sendResult.Withdraw.Amount)); ok && amount > 0 {
			record.Amount = amount
		}
	}

	if err := f.withdrawQuoteRepo.Upsert(ctx, record); err != nil {
		logger.L().Errorf(
			"Sifang persist withdraw quote failed: merchant_id=%d, withdraw_no=%s, order_no=%s, err=%v",
			record.MerchantID, record.WithdrawNo, record.OrderNo, err,
		)
	}
}

func wrapResponse(text string) *types.Response {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return &types.Response{Text: text}
}

func (f *Feature) handleChannelRates(ctx context.Context, merchantID int64) (string, bool, error) {
	statuses, err := f.paymentService.GetChannelStatus(ctx, merchantID)
	if err != nil {
		logger.L().Errorf("Sifang channel status query failed: merchant_id=%d, err=%v", merchantID, err)
		return fmt.Sprintf("❌ 查询费率失败：%v", err), true, nil
	}

	if len(statuses) == 0 {
		return "ℹ️ 暂无通道状态数据", true, nil
	}

	message := formatChannelRatesMessage(statuses)
	logger.L().Infof("Sifang channel status queried: merchant_id=%d, channels=%d", merchantID, len(statuses))
	return message, true, nil
}

func formatChannelRatesMessage(items []*paymentservice.ChannelStatus) string {
	if len(items) == 0 {
		return "ℹ️ 暂无通道状态数据"
	}

	var sb strings.Builder
	sb.WriteString("📡 通道费率\n")
	sb.WriteString("<pre>")
	sb.WriteString("状态  通道代码    费率   通道名称\n")
	sb.WriteString("———————————————————————————————\n")

	for _, item := range items {
		if item == nil {
			continue
		}

		originalCode := strings.TrimSpace(item.ChannelCode)
		if strings.HasSuffix(strings.ToLower(originalCode), "test") {
			continue
		}

		status := "❌"
		if item.SystemEnabled && item.MerchantEnabled {
			status = "✅"
		}

		code := originalCode
		if code == "" {
			code = "-"
		}
		name := strings.TrimSpace(item.ChannelName)
		if name == "" {
			name = "-"
		}

		rate := formatChannelRate(item.Rate)

		line := fmt.Sprintf("%s %-8s %-6s %s\n",
			status,
			html.EscapeString(code),
			html.EscapeString(rate),
			html.EscapeString(name),
		)
		sb.WriteString(line)
	}

	output := strings.TrimRight(sb.String(), "\n")
	return output + "\n</pre>"
}

func formatChannelRate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return "-"
	}

	hasPercent := strings.ContainsAny(raw, "%％")
	normalized := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(raw, "%"), "％"))
	normalized = strings.ReplaceAll(normalized, ",", "")
	if normalized == "" {
		return "-"
	}

	if value, err := strconv.ParseFloat(normalized, 64); err == nil {
		if hasPercent || value > 1 {
			return strconv.FormatFloat(value, 'f', -1, 64) + "%"
		}
		return strconv.FormatFloat(value*100, 'f', -1, 64) + "%"
	}

	return raw
}
