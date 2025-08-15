package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorCode 错误码类型
type ErrorCode int

const (
	// 通用错误码
	SuccessCode       ErrorCode = 0
	InternalErrorCode ErrorCode = 10001
	InvalidParamsCode ErrorCode = 10002
	NotFoundCode      ErrorCode = 10003
	UnauthorizedCode  ErrorCode = 10004
	ForbiddenCode     ErrorCode = 10005

	// 业务错误码
	UserNotFoundCode ErrorCode = 20001
	UserExistsCode   ErrorCode = 20002
	InvalidTokenCode ErrorCode = 20003
)

// Response 统一响应结构
type Response struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    SuccessCode,
		Message: "success",
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code ErrorCode, message string) {
	statusCode := getHTTPStatusCode(code)
	c.JSON(statusCode, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData 带数据的错误响应
func ErrorWithData(c *gin.Context, code ErrorCode, message string, data interface{}) {
	statusCode := getHTTPStatusCode(code)
	c.JSON(statusCode, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// getHTTPStatusCode 根据错误码获取HTTP状态码
func getHTTPStatusCode(code ErrorCode) int {
	switch code {
	case SuccessCode:
		return http.StatusOK
	case InvalidParamsCode:
		return http.StatusBadRequest
	case UnauthorizedCode, InvalidTokenCode:
		return http.StatusUnauthorized
	case ForbiddenCode:
		return http.StatusForbidden
	case NotFoundCode, UserNotFoundCode:
		return http.StatusNotFound
	case UserExistsCode:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// ErrorMessages 错误消息映射
var ErrorMessages = map[ErrorCode]string{
	SuccessCode:       "操作成功",
	InternalErrorCode: "内部服务器错误",
	InvalidParamsCode: "参数错误",
	NotFoundCode:      "资源不存在",
	UnauthorizedCode:  "未授权",
	ForbiddenCode:     "禁止访问",
	UserNotFoundCode:  "用户不存在",
	UserExistsCode:    "用户已存在",
	InvalidTokenCode:  "无效的令牌",
}

// GetErrorMessage 获取错误消息
func GetErrorMessage(code ErrorCode) string {
	if msg, ok := ErrorMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
