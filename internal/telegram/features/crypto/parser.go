package crypto

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// 命令正则：[支付方式字母][商家序号] [金额（可选）]
	commandRegex = regexp.MustCompile(`^([azwkAZWK])([0-9])(\s+(\d+(\.\d+)?))?$`)
)

// PaymentMethodMap 支付方式字母到 API 参数的映射
var PaymentMethodMap = map[string]string{
	"a": "all",
	"A": "all",
	"z": "aliPay",
	"Z": "aliPay",
	"k": "bank",
	"K": "bank",
	"w": "wxPay",
	"W": "wxPay",
}

// PaymentMethodName 支付方式名称（中文）
var PaymentMethodName = map[string]string{
	"all":    "全部",
	"aliPay": "支付宝",
	"bank":   "银行卡",
	"wxPay":  "微信",
}

// CommandInfo 解析后的命令信息
type CommandInfo struct {
	PaymentMethodLetter string  // 支付方式字母（a/z/k/w）
	PaymentMethod       string  // 支付方式 API 参数（all/aliPay/bank/wxPay）
	PaymentMethodName   string  // 支付方式名称（全部/支付宝/银行卡/微信）
	SerialNum           int     // 商家序号（0-9，0 会自动转为 3）
	Amount              float64 // USDT 金额（可选，0 表示未提供）
	HasAmount           bool    // 是否提供了金额
}

// ParseCommand 解析命令文本
// 格式：[支付方式字母][商家序号] [金额（可选）]
// 示例：z3、z3 100、A1、k5 50
func ParseCommand(text string) (*CommandInfo, error) {
	// 去除首尾空格
	text = strings.TrimSpace(text)

	// 正则匹配
	matches := commandRegex.FindStringSubmatch(text)
	if matches == nil {
		return nil, fmt.Errorf("invalid command format")
	}

	// 提取支付方式字母
	letter := matches[1]
	paymentMethod, ok := PaymentMethodMap[letter]
	if !ok {
		return nil, fmt.Errorf("invalid payment method letter: %s", letter)
	}

	// 提取商家序号
	serialNumStr := matches[2]
	serialNum, err := strconv.Atoi(serialNumStr)
	if err != nil {
		return nil, fmt.Errorf("invalid serial number: %s", serialNumStr)
	}

	// 特殊处理：序号 0 转换为 3
	if serialNum == 0 {
		serialNum = 3
	}

	// 提取金额（可选）
	var amount float64
	hasAmount := false
	if len(matches) >= 5 && matches[4] != "" {
		amount, err = strconv.ParseFloat(matches[4], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount: %s", matches[4])
		}
		hasAmount = true
	}

	return &CommandInfo{
		PaymentMethodLetter: letter,
		PaymentMethod:       paymentMethod,
		PaymentMethodName:   PaymentMethodName[paymentMethod],
		SerialNum:           serialNum,
		Amount:              amount,
		HasAmount:           hasAmount,
	}, nil
}
