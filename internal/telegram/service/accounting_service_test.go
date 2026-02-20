package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go_bot/internal/telegram/models"
)

type accountingRangeCall struct {
	start    time.Time
	end      time.Time
	currency string
}

type stubAccountingRepository struct {
	created        *models.AccountingRecord
	dateRangeCalls []accountingRangeCall
	recentRecords  []*models.AccountingRecord
	recentErr      error
	deleteChatID   int64
	deleteRecordID string
	deleteErr      error
	deleteAllChat  int64
	deleteAllCount int64
	deleteAllErr   error
}

func (s *stubAccountingRepository) CreateRecord(ctx context.Context, record *models.AccountingRecord) error {
	s.created = record
	return nil
}

func (s *stubAccountingRepository) GetRecordsByDateRange(
	ctx context.Context,
	chatID int64,
	startTime, endTime time.Time,
	currency string,
) ([]*models.AccountingRecord, error) {
	s.dateRangeCalls = append(s.dateRangeCalls, accountingRangeCall{
		start:    startTime,
		end:      endTime,
		currency: currency,
	})

	// 仅 CNY 的当日查询返回一条 UTC 记录，用于验证展示时区转换。
	if currency == models.CurrencyCNY && endTime.Sub(startTime) == 24*time.Hour {
		return []*models.AccountingRecord{
			{
				Amount:     100,
				Currency:   models.CurrencyCNY,
				RecordedAt: time.Date(2026, 2, 16, 5, 29, 0, 0, time.UTC),
			},
		}, nil
	}

	return nil, nil
}

func (s *stubAccountingRepository) GetRecentRecords(ctx context.Context, chatID int64, days int) ([]*models.AccountingRecord, error) {
	if s.recentErr != nil {
		return nil, s.recentErr
	}
	return s.recentRecords, nil
}

func (s *stubAccountingRepository) DeleteRecord(ctx context.Context, chatID int64, recordID string) error {
	s.deleteChatID = chatID
	s.deleteRecordID = recordID
	return s.deleteErr
}

func (s *stubAccountingRepository) DeleteAllByChatID(ctx context.Context, chatID int64) (int64, error) {
	s.deleteAllChat = chatID
	if s.deleteAllErr != nil {
		return 0, s.deleteAllErr
	}
	return s.deleteAllCount, nil
}

func (s *stubAccountingRepository) EnsureIndexes(ctx context.Context) error {
	return nil
}

func TestAccountingServiceQueryRecordsFormatsRecordTimeInBeijing(t *testing.T) {
	repo := &stubAccountingRepository{}
	loc := time.FixedZone("CST", 8*3600)
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       loc,
	}

	report, err := svc.QueryRecords(context.Background(), -10001)
	if err != nil {
		t.Fatalf("QueryRecords failed: %v", err)
	}

	if !strings.Contains(report, "13:29 +100") {
		t.Fatalf("expected report to show converted Beijing time, got:\n%s", report)
	}

	if strings.Contains(report, "05:29 +100") {
		t.Fatalf("expected report not to show UTC time, got:\n%s", report)
	}
}

func TestAccountingServiceQueryRecordsUsesBeijingDayWindow(t *testing.T) {
	repo := &stubAccountingRepository{}
	loc := time.FixedZone("CST", 8*3600)
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       loc,
	}

	_, err := svc.QueryRecords(context.Background(), -10002)
	if err != nil {
		t.Fatalf("QueryRecords failed: %v", err)
	}

	var todayCalls []accountingRangeCall
	for _, call := range repo.dateRangeCalls {
		if call.end.Sub(call.start) == 24*time.Hour {
			todayCalls = append(todayCalls, call)
		}
	}

	if len(todayCalls) != 2 {
		t.Fatalf("expected 2 today range calls, got %d", len(todayCalls))
	}

	for _, call := range todayCalls {
		_, offset := call.start.Zone()
		if offset != 8*3600 {
			t.Fatalf("expected UTC+8 day window, got offset=%d", offset)
		}
		if call.start.Hour() != 0 || call.start.Minute() != 0 || call.start.Second() != 0 {
			t.Fatalf("expected day window start at 00:00:00, got %s", call.start.Format(time.RFC3339))
		}
	}
}

func TestAccountingServiceAddRecordUsesBeijingTime(t *testing.T) {
	repo := &stubAccountingRepository{}
	loc := time.FixedZone("CST", 8*3600)
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       loc,
	}

	if err := svc.AddRecord(context.Background(), -10003, 9001, "+100U"); err != nil {
		t.Fatalf("AddRecord failed: %v", err)
	}

	if repo.created == nil {
		t.Fatal("expected record to be created")
	}

	_, offset := repo.created.RecordedAt.Zone()
	if offset != 8*3600 {
		t.Fatalf("expected recorded_at in UTC+8, got offset=%d", offset)
	}
}

func TestAccountingServiceAddRecordRejectsNonPositiveAmount(t *testing.T) {
	repo := &stubAccountingRepository{}
	loc := time.FixedZone("CST", 8*3600)
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       loc,
	}

	if err := svc.AddRecord(context.Background(), -10004, 9002, "出1-2Y"); err == nil {
		t.Fatalf("expected error for negative expression result, got nil")
	}

	if repo.created != nil {
		t.Fatal("record should not be created when amount is non-positive")
	}
}

func TestAccountingServiceDeleteRecordPassesChatID(t *testing.T) {
	repo := &stubAccountingRepository{}
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       time.FixedZone("CST", 8*3600),
	}

	if err := svc.DeleteRecord(context.Background(), -20001, "abc123"); err != nil {
		t.Fatalf("DeleteRecord failed: %v", err)
	}

	if repo.deleteChatID != -20001 {
		t.Fatalf("unexpected chat id: got %d want %d", repo.deleteChatID, -20001)
	}
	if repo.deleteRecordID != "abc123" {
		t.Fatalf("unexpected record id: got %q want %q", repo.deleteRecordID, "abc123")
	}
}

func TestAccountingServiceDeleteRecordReturnsError(t *testing.T) {
	repo := &stubAccountingRepository{deleteErr: errors.New("delete failed")}
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       time.FixedZone("CST", 8*3600),
	}

	if err := svc.DeleteRecord(context.Background(), -20002, "abc456"); err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestAccountingServiceClearAllRecordsPassesChatID(t *testing.T) {
	repo := &stubAccountingRepository{deleteAllCount: 7}
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       time.FixedZone("CST", 8*3600),
	}

	deleted, err := svc.ClearAllRecords(context.Background(), -30001)
	if err != nil {
		t.Fatalf("ClearAllRecords failed: %v", err)
	}
	if deleted != 7 {
		t.Fatalf("unexpected deleted count: got %d want %d", deleted, 7)
	}
	if repo.deleteAllChat != -30001 {
		t.Fatalf("unexpected chat id: got %d want %d", repo.deleteAllChat, -30001)
	}
}

func TestAccountingServiceGetRecentRecordsForDeletion(t *testing.T) {
	repo := &stubAccountingRepository{
		recentRecords: []*models.AccountingRecord{
			{ChatID: -40001, Amount: 10},
		},
	}
	svc := &AccountingServiceImpl{
		accountingRepo: repo,
		location:       time.FixedZone("CST", 8*3600),
	}

	records, err := svc.GetRecentRecordsForDeletion(context.Background(), -40001)
	if err != nil {
		t.Fatalf("GetRecentRecordsForDeletion failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("unexpected records count: got %d want %d", len(records), 1)
	}
}
