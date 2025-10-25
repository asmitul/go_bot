package calculator

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Calculator 数学表达式计算器
// 支持基础四则运算: +, -, *, /, ()
type Calculator struct {
	expression string
	position   int
	current    rune
}

// Calculate 计算数学表达式
// 支持: 加法(+), 减法(-), 乘法(*), 除法(/), 括号(())
// 返回: 计算结果和错误信息
func Calculate(expression string) (float64, error) {
	// 移除所有空格
	expression = strings.ReplaceAll(expression, " ", "")

	if expression == "" {
		return 0, fmt.Errorf("表达式为空")
	}

	calc := &Calculator{
		expression: expression,
		position:   0,
	}

	if len(expression) > 0 {
		calc.current = rune(expression[0])
	}

	result, err := calc.parseExpression()
	if err != nil {
		return 0, err
	}

	// 检查是否还有未解析的字符
	if calc.position < len(calc.expression) {
		return 0, fmt.Errorf("表达式包含无效字符: %c", calc.current)
	}

	return result, nil
}

// advance 移动到下一个字符
func (c *Calculator) advance() {
	c.position++
	if c.position < len(c.expression) {
		c.current = rune(c.expression[c.position])
	}
}

// parseExpression 解析表达式: Term (('+' | '-') Term)*
func (c *Calculator) parseExpression() (float64, error) {
	result, err := c.parseTerm()
	if err != nil {
		return 0, err
	}

	for c.position < len(c.expression) && (c.current == '+' || c.current == '-') {
		op := c.current
		c.advance()

		term, err := c.parseTerm()
		if err != nil {
			return 0, err
		}

		if op == '+' {
			result += term
		} else {
			result -= term
		}
	}

	return result, nil
}

// parseTerm 解析项: Factor (('*' | '/') Factor)*
func (c *Calculator) parseTerm() (float64, error) {
	result, err := c.parseFactor()
	if err != nil {
		return 0, err
	}

	for c.position < len(c.expression) && (c.current == '*' || c.current == '/') {
		op := c.current
		c.advance()

		factor, err := c.parseFactor()
		if err != nil {
			return 0, err
		}

		if op == '*' {
			result *= factor
		} else {
			if factor == 0 {
				return 0, fmt.Errorf("除数不能为零")
			}
			result /= factor
		}
	}

	return result, nil
}

// parseFactor 解析因子: Number | '(' Expression ')' | '-' Factor
func (c *Calculator) parseFactor() (float64, error) {
	// 检查是否到达表达式末尾
	if c.position >= len(c.expression) {
		return 0, fmt.Errorf("表达式意外结束")
	}

	// 处理负号
	if c.current == '-' {
		c.advance()
		factor, err := c.parseFactor()
		if err != nil {
			return 0, err
		}
		return -factor, nil
	}

	// 处理正号
	if c.current == '+' {
		c.advance()
		return c.parseFactor()
	}

	// 处理括号
	if c.current == '(' {
		c.advance()
		result, err := c.parseExpression()
		if err != nil {
			return 0, err
		}

		if c.position >= len(c.expression) || c.current != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		c.advance()
		return result, nil
	}

	// 处理数字
	return c.parseNumber()
}

// parseNumber 解析数字 (整数或小数)
func (c *Calculator) parseNumber() (float64, error) {
	start := c.position

	// 读取数字部分
	for c.position < len(c.expression) && (unicode.IsDigit(c.current) || c.current == '.') {
		c.advance()
	}

	if start == c.position {
		if c.position < len(c.expression) {
			return 0, fmt.Errorf("无效的字符: %c", c.current)
		}
		return 0, fmt.Errorf("表达式结尾缺少数字")
	}

	numStr := c.expression[start:c.position]
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的数字: %s", numStr)
	}

	return num, nil
}

// IsMathExpression 判断字符串是否为数学表达式
// 只包含数字、运算符和括号,且不以运算符结尾
func IsMathExpression(text string) bool {
	// 移除空格
	text = strings.TrimSpace(text)

	// 空字符串不是数学表达式
	if text == "" {
		return false
	}

	// 检查是否只包含数字、运算符、括号和小数点
	validChars := 0
	for _, ch := range text {
		if unicode.IsDigit(ch) || ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '(' || ch == ')' || ch == '.' || ch == ' ' {
			if ch != ' ' {
				validChars++
			}
		} else {
			return false
		}
	}

	// 至少要有一个有效字符
	if validChars == 0 {
		return false
	}

	// 去除空格后的文本
	cleaned := strings.ReplaceAll(text, " ", "")

	// 必须包含至少一个数字
	hasDigit := false
	for _, ch := range cleaned {
		if unicode.IsDigit(ch) {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return false
	}

	// 至少包含一个运算符（排除纯数字）
	hasOperator := false
	for i, ch := range cleaned {
		switch ch {
		case '+', '*', '/':
			hasOperator = true
		case '-':
			if i > 0 {
				hasOperator = true
			}
		}
		if hasOperator {
			break
		}
	}
	if !hasOperator {
		return false
	}

	// 不应该以运算符结尾(除了括号)
	lastChar := rune(cleaned[len(cleaned)-1])
	if lastChar == '+' || lastChar == '-' || lastChar == '*' || lastChar == '/' {
		return false
	}

	// 不应该以左括号开始且只有一个字符
	if len(cleaned) == 1 && cleaned[0] == '(' {
		return false
	}

	return true
}
