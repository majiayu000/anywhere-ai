// Package sdk provides Go SDK for anywhere backend integration
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// AnywhereClient anywhere后端客户端
type AnywhereClient struct {
	apiKey     string
	baseURL    string
	timeout    time.Duration
	httpClient *http.Client
}

// NewAnywhereClient 创建anywhere客户端
func NewAnywhereClient(apiKey, baseURL string) *AnywhereClient {
	return &AnywhereClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		timeout: 30 * time.Second,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WithTimeout 设置超时时间
func (c *AnywhereClient) WithTimeout(timeout time.Duration) *AnywhereClient {
	c.timeout = timeout
	c.httpClient.Timeout = timeout
	return c
}

// SendMessage 发送消息
func (c *AnywhereClient) SendMessage(ctx context.Context, req *SendMessageRequest) (*SendMessageResponse, error) {
	// 如果没有提供agent_instance_id，生成一个新的
	if req.AgentInstanceID == "" {
		if req.AgentType == "" {
			return nil, fmt.Errorf("agent_type is required when creating a new instance")
		}
		req.AgentInstanceID = uuid.New().String()
	}

	// 构建请求数据
	requestData := map[string]interface{}{
		"content":             req.Content,
		"agent_instance_id":   req.AgentInstanceID,
		"requires_user_input": req.RequiresUserInput,
	}

	if req.AgentType != "" {
		requestData["agent_type"] = req.AgentType
	}
	if req.GitDiff != "" {
		requestData["git_diff"] = req.GitDiff
	}
	if req.SendPush != nil {
		requestData["send_push"] = *req.SendPush
	}
	if req.SendEmail != nil {
		requestData["send_email"] = *req.SendEmail
	}
	if req.SendSMS != nil {
		requestData["send_sms"] = *req.SendSMS
	}

	// 发送HTTP请求
	respData, err := c.makeRequest(ctx, "POST", "/api/v1/messages/agent", requestData)
	if err != nil {
		return nil, err
	}

	// 解析响应
	response := &SendMessageResponse{
		Success:         respData["success"].(bool),
		AgentInstanceID: respData["agent_instance_id"].(string),
		MessageID:       respData["message_id"].(string),
	}

	// 处理排队的用户消息
	if queuedMsgs, exists := respData["queued_user_messages"]; exists && queuedMsgs != nil {
		if msgs, ok := queuedMsgs.([]interface{}); ok {
			for _, msg := range msgs {
				if msgStr, ok := msg.(string); ok {
					response.QueuedUserMessages = append(response.QueuedUserMessages, msgStr)
				} else if msgMap, ok := msg.(map[string]interface{}); ok {
					if content, exists := msgMap["content"]; exists {
						if contentStr, ok := content.(string); ok {
							response.QueuedUserMessages = append(response.QueuedUserMessages, contentStr)
						}
					}
				}
			}
		}
	}

	// 如果需要用户输入，启动轮询
	if req.RequiresUserInput {
		return c.pollForUserInput(ctx, response, req.TimeoutMinutes, req.PollInterval)
	}

	return response, nil
}

// GetPendingMessages 获取待处理消息
func (c *AnywhereClient) GetPendingMessages(ctx context.Context, agentInstanceID string, lastReadMessageID string) (*PendingMessagesResponse, error) {
	params := map[string]string{
		"agent_instance_id": agentInstanceID,
	}
	if lastReadMessageID != "" {
		params["last_read_message_id"] = lastReadMessageID
	}

	respData, err := c.makeRequestWithParams(ctx, "GET", "/api/v1/messages/pending", params)
	if err != nil {
		return nil, err
	}

	response := &PendingMessagesResponse{
		AgentInstanceID: respData["agent_instance_id"].(string),
		Status:          respData["status"].(string),
	}

	// 解析消息
	if messagesData, exists := respData["messages"]; exists && messagesData != nil {
		if messages, ok := messagesData.([]interface{}); ok {
			for _, msgData := range messages {
				if msgMap, ok := msgData.(map[string]interface{}); ok {
					msg := &Message{
						ID:            msgMap["id"].(string),
						Content:       msgMap["content"].(string),
						SenderType:    msgMap["sender_type"].(string),
						RequiresInput: msgMap["requires_user_input"].(bool),
					}

					if gitDiff, exists := msgMap["git_diff"]; exists && gitDiff != nil {
						msg.GitDiff = gitDiff.(string)
					}

					if metadata, exists := msgMap["metadata"]; exists && metadata != nil {
						msg.Metadata = metadata.(map[string]interface{})
					}

					if createdAt, exists := msgMap["created_at"]; exists && createdAt != nil {
						if timeStr, ok := createdAt.(string); ok {
							if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
								msg.CreatedAt = parsedTime
							}
						}
					}

					response.Messages = append(response.Messages, msg)
				}
			}
		}
	}

	return response, nil
}

// RequestUserInput 请求用户输入
func (c *AnywhereClient) RequestUserInput(ctx context.Context, messageID string, timeoutMinutes int) ([]string, error) {
	// 更新消息以请求用户输入
	respData, err := c.makeRequest(ctx, "PATCH", fmt.Sprintf("/api/v1/messages/%s/request-input", messageID), nil)
	if err != nil {
		return nil, err
	}

	agentInstanceID := respData["agent_instance_id"].(string)

	// 如果已经有消息，直接返回
	if messages, exists := respData["messages"]; exists && messages != nil {
		if msgs, ok := messages.([]interface{}); ok {
			var contents []string
			for _, msg := range msgs {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					if content, exists := msgMap["content"]; exists {
						contents = append(contents, content.(string))
					}
				}
			}
			if len(contents) > 0 {
				return contents, nil
			}
		}
	}

	// 轮询等待用户响应
	return c.pollForUserResponse(ctx, agentInstanceID, messageID, timeoutMinutes)
}

// EndSession 结束会话
func (c *AnywhereClient) EndSession(ctx context.Context, agentInstanceID string) (*EndSessionResponse, error) {
	requestData := map[string]interface{}{
		"agent_instance_id": agentInstanceID,
	}

	respData, err := c.makeRequest(ctx, "POST", "/api/v1/sessions/end", requestData)
	if err != nil {
		return nil, err
	}

	return &EndSessionResponse{
		Success:         respData["success"].(bool),
		AgentInstanceID: respData["agent_instance_id"].(string),
		FinalStatus:     respData["final_status"].(string),
	}, nil
}

// SendUserMessage 发送用户消息
func (c *AnywhereClient) SendUserMessage(ctx context.Context, agentInstanceID, content string, markAsRead bool) (*SendUserMessageResponse, error) {
	requestData := map[string]interface{}{
		"agent_instance_id": agentInstanceID,
		"content":          content,
		"mark_as_read":     markAsRead,
	}

	respData, err := c.makeRequest(ctx, "POST", "/api/v1/messages/user", requestData)
	if err != nil {
		return nil, err
	}

	return &SendUserMessageResponse{
		Success:      respData["success"].(bool),
		MessageID:    respData["message_id"].(string),
		MarkedAsRead: respData["marked_as_read"].(bool),
	}, nil
}

// 私有辅助方法

// makeRequest 发送HTTP请求
func (c *AnywhereClient) makeRequest(ctx context.Context, method, endpoint string, data interface{}) (map[string]interface{}, error) {
	url := c.baseURL + endpoint

	var reqBody []byte
	var err error

	if data != nil {
		reqBody, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request data: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid API key")
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	// 解析响应
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return respData, nil
}

// makeRequestWithParams 发送带查询参数的HTTP请求
func (c *AnywhereClient) makeRequestWithParams(ctx context.Context, method, endpoint string, params map[string]string) (map[string]interface{}, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 添加查询参数
	if len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	// 解析响应
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return respData, nil
}

// pollForUserInput 轮询用户输入
func (c *AnywhereClient) pollForUserInput(ctx context.Context, response *SendMessageResponse, timeoutMinutes int, pollInterval float64) (*SendMessageResponse, error) {
	if timeoutMinutes <= 0 {
		timeoutMinutes = 1440 // 默认24小时
	}
	if pollInterval <= 0 {
		pollInterval = 3.0 // 默认3秒
	}

	timeout := time.Duration(timeoutMinutes) * time.Minute
	interval := time.Duration(pollInterval * float64(time.Second))

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
			// 轮询待处理消息
			pending, err := c.GetPendingMessages(ctx, response.AgentInstanceID, response.MessageID)
			if err != nil {
				continue // 忽略轮询错误，继续尝试
			}

			// 检查是否有新消息
			if len(pending.Messages) > 0 {
				// 提取消息内容
				for _, msg := range pending.Messages {
					response.QueuedUserMessages = append(response.QueuedUserMessages, msg.Content)
				}
				return response, nil
			}

			// 检查状态是否过期
			if pending.Status == "stale" {
				return nil, fmt.Errorf("another process has read the messages")
			}
		}
	}

	return nil, fmt.Errorf("user input timeout after %d minutes", timeoutMinutes)
}

// pollForUserResponse 轮询用户响应
func (c *AnywhereClient) pollForUserResponse(ctx context.Context, agentInstanceID, messageID string, timeoutMinutes int) ([]string, error) {
	timeout := time.Duration(timeoutMinutes) * time.Minute
	interval := 3 * time.Second

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
			// 轮询待处理消息
			pending, err := c.GetPendingMessages(ctx, agentInstanceID, messageID)
			if err != nil {
				continue
			}

			// 检查是否有新消息
			if len(pending.Messages) > 0 {
				var contents []string
				for _, msg := range pending.Messages {
					contents = append(contents, msg.Content)
				}
				return contents, nil
			}

			// 检查状态是否过期
			if pending.Status == "stale" {
				return nil, fmt.Errorf("another process has read the messages")
			}
		}
	}

	return nil, fmt.Errorf("no user response received after %d minutes", timeoutMinutes)
}

// Close 关闭客户端
func (c *AnywhereClient) Close() error {
	// HTTP客户端不需要显式关闭
	return nil
}