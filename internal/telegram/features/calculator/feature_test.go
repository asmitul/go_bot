package calculator

import (
	"context"
	"strings"
	"testing"

	botModels "github.com/go-telegram/bot/models"
	"go_bot/internal/telegram/models"
)

func TestProcess_SuccessFormatsCopyFriendlyResult(t *testing.T) {
	feature := New()
	msg := &botModels.Message{
		Text: "1+2*3",
		Chat: botModels.Chat{ID: 123},
	}

	resp, handled, err := feature.Process(context.Background(), msg, &models.Group{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if resp == nil {
		t.Fatalf("expected response")
	}
	if resp.ReplyMarkup != nil {
		t.Fatalf("expected no reply markup for calculator response")
	}

	want := "🧮 <code>1+2*3</code>\n<pre>7</pre>"
	if resp.Text != want {
		t.Fatalf("unexpected response text:\nwant: %s\ngot:  %s", want, resp.Text)
	}
}

func TestProcess_CalculationErrorReturnsTextResponse(t *testing.T) {
	feature := New()
	msg := &botModels.Message{
		Text: "(1+2",
		Chat: botModels.Chat{ID: 456},
	}

	resp, handled, err := feature.Process(context.Background(), msg, &models.Group{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if resp == nil {
		t.Fatalf("expected response")
	}
	if resp.ReplyMarkup != nil {
		t.Fatalf("expected no reply markup for calculator response")
	}
	if got := resp.Text; !strings.HasPrefix(got, "❌ 计算错误") {
		t.Fatalf("unexpected error response text: %s", got)
	}
}
