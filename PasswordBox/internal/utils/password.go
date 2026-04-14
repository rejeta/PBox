// Package utils 提供工具函数
package utils

import (
	"fmt"
	"strings"
	"unicode"
)

// PasswordStrength 密码强度信息
type PasswordStrength struct {
	Score       int      `json:"score"`       // 0-5 分
	Level       string   `json:"level"`       // "弱"/"中"/"强"
	Suggestions []string `json:"suggestions"` // 改进建议
}

// CheckPasswordStrength 检查密码强度
func CheckPasswordStrength(password string) PasswordStrength {
	var score int
	var suggestions []string

	// 1. 长度检查
	length := len(password)
	switch {
	case length < 8:
		suggestions = append(suggestions, "密码长度至少8位")
	case length >= 12:
		score += 2
	case length >= 8:
		score++
	}

	// 2. 字符类型检查
	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}

	// 每种类别加1分
	if hasUpper {
		score++
	} else {
		suggestions = append(suggestions, "建议包含大写字母")
	}

	if hasLower {
		score++
	} else {
		suggestions = append(suggestions, "建议包含小写字母")
	}

	if hasDigit {
		score++
	} else {
		suggestions = append(suggestions, "建议包含数字")
	}

	if hasSpecial {
		score++
	} else {
		suggestions = append(suggestions, "建议包含特殊字符（如 !@#$%）")
	}

	// 3. 常见弱密码检查（完全匹配或极短包含）
	commonPasswords := []string{"123456", "password", "qwerty", "111111"}
	lowerPwd := strings.ToLower(password)
	for _, common := range commonPasswords {
		// 完全匹配或者是短密码的简单组合
		if lowerPwd == common || (len(password) < 10 && strings.Contains(lowerPwd, common)) {
			score = 0
			suggestions = append(suggestions, "使用了常见弱密码模式")
			break
		}
	}

	// 4. 连续字符检查
	if hasSequential(password) {
		score--
		suggestions = append(suggestions, "避免使用连续字符（如 123、abc）")
	}

	// 5. 重复字符检查
	if hasRepeating(password) {
		score--
		suggestions = append(suggestions, "避免使用重复字符（如 aaa、111）")
	}

	// 确保分数在 0-5 范围内
	if score < 0 {
		score = 0
	}
	if score > 5 {
		score = 5
	}

	// 确定等级
	level := "弱"
	if score >= 4 {
		level = "强"
	} else if score >= 2 {
		level = "中"
	}

	return PasswordStrength{
		Score:       score,
		Level:       level,
		Suggestions: suggestions,
	}
}

// IsStrongPassword 判断是否为强密码（用于强制要求）
func IsStrongPassword(password string) bool {
	strength := CheckPasswordStrength(password)
	return strength.Score >= 2 // 至少中等强度
}

// hasSequential 检查是否有连续字符（如 123、abc）
// 只检查3个及以上连续字符，且不与其他复杂字符混合
func hasSequential(s string) bool {
	if len(s) < 6 { // 短密码不检查连续字符，避免误报
		return false
	}

	consecutiveCount := 0
	for i := 0; i < len(s)-1; i++ {
		// 检查是否为连续数字
		if isDigit(s[i]) && isDigit(s[i+1]) && s[i+1] == s[i]+1 {
			consecutiveCount++
			if consecutiveCount >= 2 { // 3个连续字符
				return true
			}
		} else if isLetter(s[i]) && isLetter(s[i+1]) && s[i+1] == s[i]+1 {
			consecutiveCount++
			if consecutiveCount >= 2 {
				return true
			}
		} else {
			consecutiveCount = 0
		}
	}
	return false
}

// hasRepeating 检查是否有重复字符（如 aaa、111）
func hasRepeating(s string) bool {
	if len(s) < 3 {
		return false
	}

	for i := 0; i < len(s)-2; i++ {
		if s[i] == s[i+1] && s[i] == s[i+2] {
			return true
		}
	}
	return false
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// GetPasswordHint 获取密码提示信息
func GetPasswordHint(password string) string {
	if len(password) == 0 {
		return "请输入密码"
	}

	if len(password) < 8 {
		return "密码太短，建议至少8位"
	}

	strength := CheckPasswordStrength(password)
	return "密码强度: " + strength.Level
}

// EstimateCrackTime 估算破解时间（简单估算）
func EstimateCrackTime(password string) string {
	length := len(password)
	var charsetSize int

	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}

	if hasLower {
		charsetSize += 26
	}
	if hasUpper {
		charsetSize += 26
	}
	if hasDigit {
		charsetSize += 10
	}
	if hasSpecial {
		charsetSize += 32
	}

	// 计算组合数
	combinations := 1
	for i := 0; i < length; i++ {
		combinations *= charsetSize
	}

	// 假设每秒 10^9 次尝试
	seconds := combinations / 1e9

	switch {
	case seconds < 1:
		return "瞬间"
	case seconds < 60:
		return "小于1分钟"
	case seconds < 3600:
		return fmt.Sprintf("约 %d 分钟", int(seconds/60))
	case seconds < 86400:
		return fmt.Sprintf("约 %d 小时", int(seconds/3600))
	case seconds < 31536000:
		return fmt.Sprintf("约 %d 天", int(seconds/86400))
	case seconds < 3153600000:
		return fmt.Sprintf("约 %d 年", int(seconds/31536000))
	default:
		return "超过100年"
	}
}
