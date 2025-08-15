package sdk

import (
	"fmt"
	"time"
)

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	Content          string  `json:"content"`
	AgentType        string  `json:"agent_type,omitempty"`
	AgentInstanceID  string  `json:"agent_instance_id,omitempty"`
	RequiresUserInput bool   `json:"requires_user_input"`
	TimeoutMinutes   int     `json:"timeout_minutes,omitempty"`
	PollInterval     float64 `json:"poll_interval,omitempty"`
	SendPush         *bool   `json:"send_push,omitempty"`
	SendEmail        *bool   `json:"send_email,omitempty"`
	SendSMS          *bool   `json:"send_sms,omitempty"`
	GitDiff          string  `json:"git_diff,omitempty"`
}

// SendMessageResponse 发送消息响应
type SendMessageResponse struct {
	Success             bool     `json:"success"`
	AgentInstanceID     string   `json:"agent_instance_id"`
	MessageID           string   `json:"message_id"`
	QueuedUserMessages  []string `json:"queued_user_messages"`
}

// PendingMessagesResponse 待处理消息响应
type PendingMessagesResponse struct {
	AgentInstanceID string     `json:"agent_instance_id"`
	Messages        []*Message `json:"messages"`
	Status          string     `json:"status"`
}

// Message 消息模型
type Message struct {
	ID            string                 `json:"id"`
	Content       string                 `json:"content"`
	SenderType    string                 `json:"sender_type"`
	RequiresInput bool                   `json:"requires_user_input"`
	GitDiff       string                 `json:"git_diff"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
}

// EndSessionResponse 结束会话响应
type EndSessionResponse struct {
	Success         bool   `json:"success"`
	AgentInstanceID string `json:"agent_instance_id"`
	FinalStatus     string `json:"final_status"`
}

// SendUserMessageResponse 发送用户消息响应
type SendUserMessageResponse struct {
	Success      bool   `json:"success"`
	MessageID    string `json:"message_id"`
	MarkedAsRead bool   `json:"marked_as_read"`
}

// ClientError 客户端错误
type ClientError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Error 实现error接口
func (e *ClientError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("anywhere API error %d: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("anywhere API error %d: %s", e.Code, e.Message)
}

// IsAuthenticationError 检查是否是认证错误
func (e *ClientError) IsAuthenticationError() bool {
	return e.Code == 401
}

// IsTimeoutError 检查是否是超时错误
func (e *ClientError) IsTimeoutError() bool {
	return e.Code == 408 || e.Message == "timeout"
}

// NewSendMessageRequest 创建发送消息请求的便捷函数
func NewSendMessageRequest(content string) *SendMessageRequest {
	return &SendMessageRequest{
		Content:           content,
		RequiresUserInput: false,
		TimeoutMinutes:    1440, // 24小时
		PollInterval:      3.0,  // 3秒
	}
}

// WithAgentType 设置代理类型
func (r *SendMessageRequest) WithAgentType(agentType string) *SendMessageRequest {
	r.AgentType = agentType
	return r
}

// WithAgentInstanceID 设置代理实例ID
func (r *SendMessageRequest) WithAgentInstanceID(instanceID string) *SendMessageRequest {
	r.AgentInstanceID = instanceID
	return r
}

// WithUserInput 设置需要用户输入
func (r *SendMessageRequest) WithUserInput(timeout int, pollInterval float64) *SendMessageRequest {
	r.RequiresUserInput = true
	if timeout > 0 {
		r.TimeoutMinutes = timeout
	}
	if pollInterval > 0 {
		r.PollInterval = pollInterval
	}
	return r
}

// WithGitDiff 设置Git差异
func (r *SendMessageRequest) WithGitDiff(gitDiff string) *SendMessageRequest {
	r.GitDiff = gitDiff
	return r
}

// WithNotifications 设置通知选项
func (r *SendMessageRequest) WithNotifications(push, email, sms *bool) *SendMessageRequest {
	r.SendPush = push
	r.SendEmail = email
	r.SendSMS = sms
	return r
}