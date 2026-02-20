package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"sync"
	"testing"

	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type telegramAPICall struct {
	Method string
	Body   string
}

type telegramAPISpy struct {
	mu    sync.Mutex
	calls []telegramAPICall
}

func (s *telegramAPISpy) record(method, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, telegramAPICall{
		Method: method,
		Body:   body,
	})
}

func (s *telegramAPISpy) count(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for _, call := range s.calls {
		if call.Method == method {
			total++
		}
	}
	return total
}

func newTelegramTestBot(t *testing.T) (*bot.Bot, *telegramAPISpy) {
	t.Helper()

	spy := &telegramAPISpy{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		body, _ := io.ReadAll(r.Body)
		method := path.Base(r.URL.Path)
		spy.record(method, string(body))

		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "sendMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":101,"date":1700000000,"chat":{"id":-1001,"type":"group"}}}`))
		case "answerCallbackQuery":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		}
	}))
	t.Cleanup(server.Close)

	apiBot, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("create telegram bot failed: %v", err)
	}
	return apiBot, spy
}

type accountingHandlerStubUserService struct {
	isAdmin  bool
	adminErr error
}

func (s *accountingHandlerStubUserService) RegisterOrUpdateUser(ctx context.Context, info *service.TelegramUserInfo) error {
	return nil
}

func (s *accountingHandlerStubUserService) GrantAdminPermission(ctx context.Context, targetID, grantedBy int64) error {
	return nil
}

func (s *accountingHandlerStubUserService) RevokeAdminPermission(ctx context.Context, targetID, revokedBy int64) error {
	return nil
}

func (s *accountingHandlerStubUserService) GetUserInfo(ctx context.Context, telegramID int64) (*models.User, error) {
	role := models.RoleUser
	if s.isAdmin {
		role = models.RoleAdmin
	}
	return &models.User{TelegramID: telegramID, Role: role}, nil
}

func (s *accountingHandlerStubUserService) ListAllAdmins(ctx context.Context) ([]*models.User, error) {
	return nil, nil
}

func (s *accountingHandlerStubUserService) CheckOwnerPermission(ctx context.Context, telegramID int64) (bool, error) {
	return false, nil
}

func (s *accountingHandlerStubUserService) CheckAdminPermission(ctx context.Context, telegramID int64) (bool, error) {
	if s.adminErr != nil {
		return false, s.adminErr
	}
	return s.isAdmin, nil
}

func (s *accountingHandlerStubUserService) UpdateUserActivity(ctx context.Context, telegramID int64) error {
	return nil
}

type accountingHandlerStubAccountingService struct {
	addErr        error
	queryErr      error
	recentErr     error
	deleteErr     error
	clearErr      error
	queryReport   string
	recentRecords []*models.AccountingRecord
	clearCount    int64

	addCalls       int
	addChatID      int64
	addUserID      int64
	addInput       string
	queryCalls     int
	queryChatID    int64
	deleteCalls    int
	deleteChatID   int64
	deleteRecordID string
	clearCalls     int
	clearChatID    int64
}

func (s *accountingHandlerStubAccountingService) AddRecord(ctx context.Context, chatID, userID int64, input string) error {
	s.addCalls++
	s.addChatID = chatID
	s.addUserID = userID
	s.addInput = input
	return s.addErr
}

func (s *accountingHandlerStubAccountingService) QueryRecords(ctx context.Context, chatID int64) (string, error) {
	s.queryCalls++
	s.queryChatID = chatID
	if s.queryErr != nil {
		return "", s.queryErr
	}
	if s.queryReport == "" {
		return "report", nil
	}
	return s.queryReport, nil
}

func (s *accountingHandlerStubAccountingService) GetRecentRecordsForDeletion(ctx context.Context, chatID int64) ([]*models.AccountingRecord, error) {
	if s.recentErr != nil {
		return nil, s.recentErr
	}
	return s.recentRecords, nil
}

func (s *accountingHandlerStubAccountingService) DeleteRecord(ctx context.Context, chatID int64, recordID string) error {
	s.deleteCalls++
	s.deleteChatID = chatID
	s.deleteRecordID = recordID
	return s.deleteErr
}

func (s *accountingHandlerStubAccountingService) ClearAllRecords(ctx context.Context, chatID int64) (int64, error) {
	s.clearCalls++
	s.clearChatID = chatID
	if s.clearErr != nil {
		return 0, s.clearErr
	}
	return s.clearCount, nil
}

func TestHandleAccountingInputSuccess(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	b := &Bot{
		bot: apiBot,
		groupService: &autoLookupTestGroupService{
			group: &models.Group{
				TelegramID: -1001,
				Type:       "group",
				Title:      "test-group",
				Settings: models.GroupSettings{
					AccountingEnabled: true,
				},
			},
		},
		userService:       &accountingHandlerStubUserService{isAdmin: true},
		accountingService: &accountingHandlerStubAccountingService{queryReport: "ok"},
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   11,
			Text: "+100U",
			Chat: botModels.Chat{
				ID:    -1001,
				Type:  "group",
				Title: "test-group",
			},
			From: &botModels.User{ID: 9001},
		},
	}

	handled := b.handleAccountingInput(context.Background(), apiBot, update)
	if !handled {
		t.Fatalf("expected accounting input to be handled")
	}

	accSvc, ok := b.accountingService.(*accountingHandlerStubAccountingService)
	if !ok {
		t.Fatalf("unexpected accounting service type")
	}
	if accSvc.addCalls != 1 {
		t.Fatalf("expected AddRecord to be called once, got %d", accSvc.addCalls)
	}
	if accSvc.addChatID != -1001 || accSvc.addUserID != 9001 || accSvc.addInput != "+100U" {
		t.Fatalf("unexpected AddRecord args: chat=%d user=%d input=%q", accSvc.addChatID, accSvc.addUserID, accSvc.addInput)
	}
	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected sendMessage to be called")
	}
}

func TestHandleQueryAccountingFeatureDisabled(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	b := &Bot{
		bot: apiBot,
		groupService: &autoLookupTestGroupService{
			group: &models.Group{
				TelegramID: -1002,
				Type:       "group",
				Title:      "disabled-group",
				Settings: models.GroupSettings{
					AccountingEnabled: false,
				},
			},
		},
		accountingService: &accountingHandlerStubAccountingService{},
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   12,
			Text: "查询记账",
			Chat: botModels.Chat{
				ID:    -1002,
				Type:  "group",
				Title: "disabled-group",
			},
			From: &botModels.User{ID: 9002},
		},
	}

	b.handleQueryAccounting(context.Background(), apiBot, update)

	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected sendMessage to be called for error response")
	}
}

func TestHandleDeleteAccountingNoRecords(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	b := &Bot{
		bot: apiBot,
		groupService: &autoLookupTestGroupService{
			group: &models.Group{
				TelegramID: -1003,
				Type:       "group",
				Title:      "delete-group",
				Settings: models.GroupSettings{
					AccountingEnabled: true,
				},
			},
		},
		accountingService: &accountingHandlerStubAccountingService{
			recentRecords: nil,
		},
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   13,
			Text: "删除记账记录",
			Chat: botModels.Chat{
				ID:    -1003,
				Type:  "group",
				Title: "delete-group",
			},
			From: &botModels.User{ID: 9003},
		},
	}

	b.handleDeleteAccounting(context.Background(), apiBot, update)

	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected sendMessage to be called when no records")
	}
}

func TestHandleClearAccountingSuccess(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	accSvc := &accountingHandlerStubAccountingService{clearCount: 3}
	b := &Bot{
		bot: apiBot,
		groupService: &autoLookupTestGroupService{
			group: &models.Group{
				TelegramID: -1004,
				Type:       "group",
				Title:      "clear-group",
				Settings: models.GroupSettings{
					AccountingEnabled: true,
				},
			},
		},
		accountingService: accSvc,
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   14,
			Text: "清零记账",
			Chat: botModels.Chat{
				ID:    -1004,
				Type:  "group",
				Title: "clear-group",
			},
			From: &botModels.User{ID: 9004},
		},
	}

	b.handleClearAccounting(context.Background(), apiBot, update)

	if accSvc.clearCalls != 1 || accSvc.clearChatID != -1004 {
		t.Fatalf("unexpected clear args: calls=%d chat=%d", accSvc.clearCalls, accSvc.clearChatID)
	}
	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected success message to be sent")
	}
}

func TestHandleAccountingDeleteCallbackRejectsNonAdmin(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)
	accSvc := &accountingHandlerStubAccountingService{}

	b := &Bot{
		bot:               apiBot,
		userService:       &accountingHandlerStubUserService{isAdmin: false},
		accountingService: accSvc,
	}

	recordID := primitive.NewObjectID().Hex()
	update := &botModels.Update{
		CallbackQuery: &botModels.CallbackQuery{
			ID:   "cb-1",
			Data: fmt.Sprintf("acc_del:%s", recordID),
			From: botModels.User{ID: 1001},
			Message: botModels.MaybeInaccessibleMessage{
				Message: &botModels.Message{
					ID: 21,
					Chat: botModels.Chat{
						ID:   -1005,
						Type: "group",
					},
				},
			},
		},
	}

	b.handleAccountingDeleteCallback(context.Background(), apiBot, update)

	if accSvc.deleteCalls != 0 {
		t.Fatalf("expected DeleteRecord not to be called, got %d", accSvc.deleteCalls)
	}
	if spy.count("answerCallbackQuery") == 0 {
		t.Fatalf("expected callback answer for rejected operation")
	}
}

func TestHandleAccountingDeleteCallbackInaccessibleMessage(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)
	accSvc := &accountingHandlerStubAccountingService{}

	b := &Bot{
		bot:               apiBot,
		userService:       &accountingHandlerStubUserService{isAdmin: true},
		accountingService: accSvc,
	}

	update := &botModels.Update{
		CallbackQuery: &botModels.CallbackQuery{
			ID:   "cb-2",
			Data: "acc_del:507f1f77bcf86cd799439011",
			From: botModels.User{ID: 1002},
			Message: botModels.MaybeInaccessibleMessage{
				Message: nil,
			},
		},
	}

	b.handleAccountingDeleteCallback(context.Background(), apiBot, update)

	if accSvc.deleteCalls != 0 {
		t.Fatalf("expected DeleteRecord not to be called for inaccessible message")
	}
	if spy.count("answerCallbackQuery") == 0 {
		t.Fatalf("expected callback answer for inaccessible message")
	}
}

func TestHandleAccountingDeleteCallbackSuccess(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)
	accSvc := &accountingHandlerStubAccountingService{
		queryReport: "latest-report",
	}

	b := &Bot{
		bot:               apiBot,
		userService:       &accountingHandlerStubUserService{isAdmin: true},
		accountingService: accSvc,
	}

	recordID := primitive.NewObjectID().Hex()
	update := &botModels.Update{
		CallbackQuery: &botModels.CallbackQuery{
			ID:   "cb-3",
			Data: fmt.Sprintf("acc_del:%s", recordID),
			From: botModels.User{ID: 1003},
			Message: botModels.MaybeInaccessibleMessage{
				Message: &botModels.Message{
					ID: 22,
					Chat: botModels.Chat{
						ID:   -1006,
						Type: "group",
					},
				},
			},
		},
	}

	b.handleAccountingDeleteCallback(context.Background(), apiBot, update)

	if accSvc.deleteCalls != 1 {
		t.Fatalf("expected DeleteRecord to be called once, got %d", accSvc.deleteCalls)
	}
	if accSvc.deleteChatID != -1006 {
		t.Fatalf("unexpected delete chat id: got %d want %d", accSvc.deleteChatID, -1006)
	}
	if accSvc.deleteRecordID != recordID {
		t.Fatalf("unexpected delete record id: got %q want %q", accSvc.deleteRecordID, recordID)
	}
	if spy.count("answerCallbackQuery") == 0 {
		t.Fatalf("expected callback answer on success")
	}
	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected updated report to be sent")
	}
}

func TestFormatRecordAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		currency string
		expected string
	}{
		{name: "usd integer income", amount: 12, currency: models.CurrencyUSD, expected: "+12U"},
		{name: "cny decimal expense", amount: -5.25, currency: models.CurrencyCNY, expected: "-5.25Y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecordAmount(tt.amount, tt.currency)
			if got != tt.expected {
				t.Fatalf("unexpected format: got %q want %q", got, tt.expected)
			}
		})
	}
}
