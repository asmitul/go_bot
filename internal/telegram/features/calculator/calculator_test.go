package calculator

import (
	"math"
	"testing"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		expected   float64
		shouldErr  bool
	}{
		// 基础加法
		{"简单加法", "1+2", 3, false},
		{"多项加法", "1+2+3", 6, false},
		{"小数加法", "1.5+2.5", 4, false},

		// 基础减法
		{"简单减法", "5-3", 2, false},
		{"负数结果", "3-5", -2, false},
		{"小数减法", "5.5-2.3", 3.2, false},

		// 基础乘法
		{"简单乘法", "3*4", 12, false},
		{"小数乘法", "2.5*4", 10, false},

		// 基础除法
		{"简单除法", "10/2", 5, false},
		{"小数除法", "7.5/2.5", 3, false},
		{"除零错误", "5/0", 0, true},

		// 括号优先级
		{"括号优先", "(1+2)*3", 9, false},
		{"嵌套括号", "((1+2)*3)+4", 13, false},
		{"复杂括号", "(10-5)*(2+3)", 25, false},

		// 运算符优先级
		{"乘法优先", "2+3*4", 14, false},
		{"除法优先", "10-6/2", 7, false},
		{"混合运算", "2+3*4-6/2", 11, false},

		// 负数处理
		{"负数开头", "-5+3", -2, false},
		{"负数括号", "(-5+3)*2", -4, false},
		{"双重负号", "--5", 5, false},

		// 正号处理
		{"正号开头", "+5+3", 8, false},
		{"正号括号", "(+5)*2", 10, false},

		// 空格处理
		{"带空格", "1 + 2 * 3", 7, false},
		{"多个空格", "  10  -  5  ", 5, false},

		// 错误情况
		{"空表达式", "", 0, true},
		{"括号不匹配", "(1+2", 0, true},
		{"多余右括号", "1+2)", 0, true},
		{"无效字符", "1+2a", 0, true},
		{"只有运算符", "+-*/", 0, true},
		{"运算符结尾", "1+2+", 0, true},

		// 复杂表达式
		{"复杂表达式1", "(2+3)*(4-1)/3", 5, false},
		{"复杂表达式2", "10+20*3-4/2", 68, false},
		{"复杂表达式3", "((10+5)*2-6)/4", 6, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Calculate(tt.expression)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("期望出错，但成功计算: %s = %f", tt.expression, result)
				}
			} else {
				if err != nil {
					t.Errorf("计算失败: %s, 错误: %v", tt.expression, err)
				} else if math.Abs(result-tt.expected) > 0.0001 {
					t.Errorf("计算结果错误: %s = %f, 期望 %f", tt.expression, result, tt.expected)
				}
			}
		})
	}
}

func TestIsMathExpression(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 有效的数学表达式
		{"简单加法", "1+2", true},
		{"简单减法", "5-3", true},
		{"简单乘法", "3*4", true},
		{"简单除法", "10/2", true},
		{"带括号", "(1+2)*3", true},
		{"小数", "1.5+2.5", true},
		{"负数", "-5+3", true},
		{"带空格", "1 + 2", true},
		{"复杂表达式", "(10+5)*2-3", true},

		// 无效的表达式
		{"空字符串", "", false},
		{"纯文字", "hello", false},
		{"文字加数字", "abc123", false},
		{"包含字母", "1+2a", false},
		{"只有运算符", "+-*/", false},
		{"以运算符结尾", "1+2+", false},
		{"只有括号", "()", false},
		{"只有空格", "   ", false},
		{"特殊字符", "1+2@", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMathExpression(tt.input)
			if result != tt.expected {
				t.Errorf("IsMathExpression(%q) = %v, 期望 %v", tt.input, result, tt.expected)
			}
		})
	}
}
