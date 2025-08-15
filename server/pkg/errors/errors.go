package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// AppError 应用错误类型
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
	Cause   error  `json:"-"`
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap 实现errors.Unwrap接口
func (e *AppError) Unwrap() error {
	return e.Cause
}

// New 创建新的应用错误
func New(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Stack:   getStack(),
	}
}

// Wrap 包装现有错误
func Wrap(err error, code int, message string) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Code:    code,
		Message: message,
		Stack:   getStack(),
		Cause:   err,
	}
}

// Wrapf 格式化包装错误
func Wrapf(err error, code int, format string, args ...interface{}) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Stack:   getStack(),
		Cause:   err,
	}
}

// getStack 获取调用栈
func getStack() string {
	var stack []string
	for i := 2; i < 10; i++ { // 跳过当前函数和调用者
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// 只显示项目相关的文件
		if strings.Contains(file, "gin-starter") {
			stack = append(stack, fmt.Sprintf("%s:%d", file, line))
		}
	}
	return strings.Join(stack, "\n")
}

// 预定义错误
var (
	ErrInternalServer = New(500, "内部服务器错误")
	ErrInvalidParams  = New(400, "参数错误")
	ErrNotFound       = New(404, "资源不存在")
	ErrUnauthorized   = New(401, "未授权")
	ErrForbidden      = New(403, "禁止访问")
)
