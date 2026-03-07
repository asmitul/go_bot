package telegram

import (
	"strings"
	"testing"
)

func TestBuildCanceledSendMoneyQuoteText_ReplacesConfirmationLine(t *testing.T) {
	original := strings.Join([]string{
		"OTC商家实时价格",
		"",
		"信息来源: 欧易 支付宝",
		"",
		"7.02 ✖️ 100 U 🟰 702.00 ¥",
		"是否确认下发 702 元 | 2023100",
		"🔐 将附带当前谷歌验证码",
	}, "\n")
	cancelText := "已取消下发 <code>702</code> 元给商户 <code>2023100</code>"

	got := buildCanceledSendMoneyQuoteText(original, cancelText)
	if strings.Contains(got, "是否确认下发") {
		t.Fatalf("expected confirmation line removed, got %q", got)
	}
	if !strings.Contains(got, "❌ "+cancelText) {
		t.Fatalf("expected cancel line injected, got %q", got)
	}
	if !strings.Contains(got, "🔐 将附带当前谷歌验证码") {
		t.Fatalf("expected other lines kept, got %q", got)
	}
}

func TestBuildCanceledSendMoneyQuoteText_AppendsWhenConfirmationMissing(t *testing.T) {
	original := "报价快照"
	cancelText := "已取消下发 <code>100</code> 元给商户 <code>2023100</code>"

	got := buildCanceledSendMoneyQuoteText(original, cancelText)
	expected := original + "\n❌ " + cancelText
	if got != expected {
		t.Fatalf("unexpected text:\nwant: %q\ngot:  %q", expected, got)
	}
}
