package sifang

import (
	"regexp"
	"strings"
	"unicode"
)

var orderNumberRegexp = regexp.MustCompile(`(?i)\b[a-z0-9]{6,32}\b`)

// ExtractOrderNumbers 从多个字符串片段中提取订单号，支持字母+数字组合并去重。
// 返回结果按照首次出现顺序排序。
func ExtractOrderNumbers(parts ...string) []string {
	seen := make(map[string]struct{})
	var results []string

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		matches := orderNumberRegexp.FindAllString(part, -1)
		if len(matches) == 0 {
			continue
		}

		for _, match := range matches {
			if !containsLetterAndDigit(match) {
				continue
			}

			normalized := strings.ToUpper(match)
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			results = append(results, match)
		}
	}

	return results
}

// NormalizeFileName 去除文件扩展名并替换常见分隔符，便于后续提取订单号。
func NormalizeFileName(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}

	trimmed := strings.TrimSpace(name)
	if dot := strings.LastIndex(trimmed, "."); dot > 0 {
		trimmed = trimmed[:dot]
	}

	replacer := strings.NewReplacer("_", " ", "-", " ")
	return replacer.Replace(trimmed)
}

func containsLetterAndDigit(s string) bool {
	hasLetter := false
	hasDigit := false

	for _, r := range s {
		if unicode.IsDigit(r) {
			hasDigit = true
		} else if unicode.IsLetter(r) {
			hasLetter = true
		}

		if hasLetter && hasDigit {
			return true
		}
	}

	return false
}
