package participle

import (
	"regexp"
	"unicode/utf8"
)

// 按Unicode字符分割字符串
func SplitString(s string) []string {
	var result []string
	for len(s) > 0 {
		_, size := utf8.DecodeRuneInString(s)
		result = append(result, s[:size])
		s = s[size:]
	}
	return result
}

// IsSpecialChar 判断字符串是否为特殊符号
func IsSpecialChar(s string) bool {
	// 检查是否为空字符串
	if s == "" {
		return false
	}

	// 使用正则表达式检查是否为标点符号或特殊符号
	chinesePunctuation := regexp.MustCompile(`[\p{P}\p{S}\p{Z}]+`)

	// 如果整个字符串都是特殊符号，则返回true
	return chinesePunctuation.MatchString(s)
}
