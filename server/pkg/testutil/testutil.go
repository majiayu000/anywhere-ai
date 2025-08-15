package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestRouter 创建测试路由
func TestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// MakeRequest 创建HTTP请求
func MakeRequest(method, url string, body interface{}) (*http.Request, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// PerformRequest 执行HTTP请求
func PerformRequest(r http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	req, _ := MakeRequest(method, path, body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// PerformRequestWithHeaders 执行带头部的HTTP请求
func PerformRequestWithHeaders(r http.Handler, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	req, _ := MakeRequest(method, path, body)

	// 设置头部
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// AssertJSON 断言JSON响应
func AssertJSON(t *testing.T, w *httptest.ResponseRecorder, expected interface{}) {
	var actual interface{}
	err := json.Unmarshal(w.Body.Bytes(), &actual)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

// AssertStatusCode 断言状态码
func AssertStatusCode(t *testing.T, w *httptest.ResponseRecorder, expectedCode int) {
	assert.Equal(t, expectedCode, w.Code)
}

// AssertContains 断言响应包含指定内容
func AssertContains(t *testing.T, w *httptest.ResponseRecorder, expected string) {
	assert.Contains(t, w.Body.String(), expected)
}

// AssertHeader 断言响应头
func AssertHeader(t *testing.T, w *httptest.ResponseRecorder, key, expected string) {
	assert.Equal(t, expected, w.Header().Get(key))
}

// ParseJSONResponse 解析JSON响应
func ParseJSONResponse(w *httptest.ResponseRecorder, v interface{}) error {
	return json.Unmarshal(w.Body.Bytes(), v)
}

// MockData 模拟数据生成器
type MockData struct{}

// NewMockData 创建模拟数据生成器
func NewMockData() *MockData {
	return &MockData{}
}

// User 模拟用户数据
func (m *MockData) User() map[string]interface{} {
	return map[string]interface{}{
		"id":         1,
		"username":   "testuser",
		"email":      "test@example.com",
		"name":       "Test User",
		"avatar":     "https://example.com/avatar.jpg",
		"created_at": "2023-01-01T00:00:00Z",
		"updated_at": "2023-01-01T00:00:00Z",
	}
}

// Users 模拟用户列表数据
func (m *MockData) Users(count int) []map[string]interface{} {
	users := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		users[i] = map[string]interface{}{
			"id":         i + 1,
			"username":   fmt.Sprintf("user%d", i+1),
			"email":      fmt.Sprintf("user%d@example.com", i+1),
			"name":       fmt.Sprintf("User %d", i+1),
			"avatar":     "https://example.com/avatar.jpg",
			"created_at": "2023-01-01T00:00:00Z",
			"updated_at": "2023-01-01T00:00:00Z",
		}
	}
	return users
}

// ErrorResponse 模拟错误响应
func (m *MockData) ErrorResponse(code int, message string) map[string]interface{} {
	return map[string]interface{}{
		"code":    code,
		"message": message,
	}
}

// SuccessResponse 模拟成功响应
func (m *MockData) SuccessResponse(data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    data,
	}
}

// PaginatedResponse 模拟分页响应
func (m *MockData) PaginatedResponse(data interface{}, page, pageSize, total int) map[string]interface{} {
	return map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"items":       data,
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + pageSize - 1) / pageSize,
		},
	}
}

// TestCase 测试用例结构
type TestCase struct {
	Name           string
	Method         string
	URL            string
	Body           interface{}
	Headers        map[string]string
	ExpectedCode   int
	ExpectedBody   interface{}
	ExpectedHeader map[string]string
	Setup          func()
	Teardown       func()
}

// RunTestCases 运行测试用例
func RunTestCases(t *testing.T, router http.Handler, testCases []TestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// 执行设置
			if tc.Setup != nil {
				tc.Setup()
			}

			// 执行清理
			if tc.Teardown != nil {
				defer tc.Teardown()
			}

			// 执行请求
			var w *httptest.ResponseRecorder
			if tc.Headers != nil {
				w = PerformRequestWithHeaders(router, tc.Method, tc.URL, tc.Body, tc.Headers)
			} else {
				w = PerformRequest(router, tc.Method, tc.URL, tc.Body)
			}

			// 断言状态码
			AssertStatusCode(t, w, tc.ExpectedCode)

			// 断言响应体
			if tc.ExpectedBody != nil {
				AssertJSON(t, w, tc.ExpectedBody)
			}

			// 断言响应头
			for key, expected := range tc.ExpectedHeader {
				AssertHeader(t, w, key, expected)
			}
		})
	}
}

// BenchmarkRequest 性能测试请求
func BenchmarkRequest(b *testing.B, router http.Handler, method, path string, body interface{}) {
	req, _ := MakeRequest(method, path, body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
