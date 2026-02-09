package forward

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-telegram/bot"
)

func TestShouldRetryForward(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "too many requests retryable",
			err: &bot.TooManyRequestsError{
				Message:    "too many requests",
				RetryAfter: 3,
			},
			want: true,
		},
		{
			name: "forbidden non retryable",
			err:  fmt.Errorf("%w, bot was kicked", bot.ErrorForbidden),
			want: false,
		},
		{
			name: "bad request non retryable",
			err:  fmt.Errorf("%w, chat not found", bot.ErrorBadRequest),
			want: false,
		},
		{
			name: "migrate error non retryable",
			err: &bot.MigrateError{
				Message:         "bad request: group upgraded",
				MigrateToChatID: -1001234567890,
			},
			want: false,
		},
		{
			name: "unauthorized non retryable",
			err:  fmt.Errorf("%w, invalid token", bot.ErrorUnauthorized),
			want: false,
		},
		{
			name: "not found non retryable",
			err:  fmt.Errorf("%w, endpoint missing", bot.ErrorNotFound),
			want: false,
		},
		{
			name: "generic error retryable",
			err:  errors.New("temporary network error"),
			want: true,
		},
		{
			name: "nil error non retryable",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetryForward(tt.err)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestMigrateToChatIDFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int64
		ok   bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: 0,
			ok:   false,
		},
		{
			name: "not migrate error",
			err:  errors.New("temporary network error"),
			want: 0,
			ok:   false,
		},
		{
			name: "migrate error",
			err: &bot.MigrateError{
				Message:         "bad request: group chat was upgraded",
				MigrateToChatID: -1003848752937,
			},
			want: -1003848752937,
			ok:   true,
		},
		{
			name: "wrapped migrate error",
			err: fmt.Errorf("wrap: %w", &bot.MigrateError{
				Message:         "bad request: group chat was upgraded",
				MigrateToChatID: -1005006007008,
			}),
			want: -1005006007008,
			ok:   true,
		},
		{
			name: "migrate error with zero id",
			err: &bot.MigrateError{
				Message:         "bad request: group chat was upgraded",
				MigrateToChatID: 0,
			},
			want: 0,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := migrateToChatIDFromError(tt.err)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("expected (%d, %v), got (%d, %v)", tt.want, tt.ok, got, ok)
			}
		})
	}
}

func TestCalculateForwardRetryDelay_TooManyRequests(t *testing.T) {
	err := &bot.TooManyRequestsError{
		Message:    "too many requests",
		RetryAfter: 4,
	}

	got := calculateForwardRetryDelay(err, 1, 123)
	want := 4*time.Second + 800*time.Millisecond // 123%5=3 => (3+1)*200ms
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestCalculateForwardRetryDelay_TooManyRequestsFallback(t *testing.T) {
	err := &bot.TooManyRequestsError{
		Message:    "too many requests",
		RetryAfter: 0,
	}

	got := calculateForwardRetryDelay(err, 1, 1)
	want := defaultForwardRetryDelay + 400*time.Millisecond // 1%5=1 => (1+1)*200ms
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestCalculateForwardRetryDelay_ExponentialBackoff(t *testing.T) {
	err := errors.New("temporary error")

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{name: "attempt 1", attempt: 1, want: 1 * time.Second},
		{name: "attempt 2", attempt: 2, want: 2 * time.Second},
		{name: "attempt 4", attempt: 4, want: 8 * time.Second},
		{name: "attempt 6 capped", attempt: 6, want: maxForwardExponentialBackoff},
		{name: "attempt 0 normalized", attempt: 0, want: 1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateForwardRetryDelay(err, tt.attempt, 100)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestForwardRetryJitter(t *testing.T) {
	tests := []struct {
		name    string
		groupID int64
		want    time.Duration
	}{
		{name: "positive id", groupID: 6, want: 400 * time.Millisecond},
		{name: "negative id", groupID: -6, want: 400 * time.Millisecond},
		{name: "zero id", groupID: 0, want: 200 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := forwardRetryJitter(tt.groupID)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
