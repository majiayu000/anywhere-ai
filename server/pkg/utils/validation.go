package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// IsEmail 验证邮箱格式
func IsEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// IsPhone 验证手机号格式（中国大陆）
func IsPhone(phone string) bool {
	phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)
	return phoneRegex.MatchString(phone)
}

// IsPassword 验证密码强度
// 至少8位，包含大小写字母、数字和特殊字符
func IsPassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}

// IsURL 验证URL格式
func IsURL(url string) bool {
	urlRegex := regexp.MustCompile(`^https?://[\w\-]+(\.[\w\-]+)+([\w\-\.,@?^=%&:/~\+#]*[\w\-\@?^=%&/~\+#])?$`)
	return urlRegex.MatchString(url)
}

// IsIDCard 验证身份证号码（中国大陆）
func IsIDCard(idCard string) bool {
	idCardRegex := regexp.MustCompile(`^[1-9]\d{5}(18|19|20)\d{2}((0[1-9])|(1[0-2]))(([0-2][1-9])|10|20|30|31)\d{3}[0-9Xx]$`)
	return idCardRegex.MatchString(idCard)
}

// IsChinese 验证是否为中文字符
func IsChinese(text string) bool {
	for _, r := range text {
		if !unicode.Is(unicode.Scripts["Han"], r) {
			return false
		}
	}
	return len(text) > 0
}

// IsAlphaNumeric 验证是否只包含字母和数字
func IsAlphaNumeric(text string) bool {
	alphaNumericRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	return alphaNumericRegex.MatchString(text)
}

// IsLength 验证字符串长度
func IsLength(text string, min, max int) bool {
	length := len([]rune(text))
	return length >= min && length <= max
}

// IsNotEmpty 验证字符串非空
func IsNotEmpty(text string) bool {
	return strings.TrimSpace(text) != ""
}

// IsIn 验证值是否在指定列表中
func IsIn(value string, list []string) bool {
	for _, item := range list {
		if value == item {
			return true
		}
	}
	return false
}

// IsIP 验证IP地址格式
func IsIP(ip string) bool {
	ipRegex := regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)
	return ipRegex.MatchString(ip)
}

// IsJSON 验证JSON格式
func IsJSON(str string) bool {
	str = strings.TrimSpace(str)
	return (strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}")) ||
		(strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]"))
}

// ValidationError 验证错误结构
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// NewValidationResult 创建验证结果
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}
}

// AddError 添加验证错误
func (vr *ValidationResult) AddError(field, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// HasErrors 检查是否有错误
func (vr *ValidationResult) HasErrors() bool {
	return !vr.Valid
}
