package anthropic

import "encoding/json"

// Anthropic Messages API 请求
type MessagesRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	System      string    `json:"system,omitempty"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	StopSequences []string `json:"stop_sequences,omitempty"`
}

// Message 消息
type Message struct {
	Role    string        `json:"role"` // user / assistant
	Content []ContentBlock `json:"content"`
}

// ContentBlock 内容块（多态）
// 使用自定义 MarshalJSON 按 Type 输出不同字段，避免序列化冲突
type ContentBlock struct {
	Type string `json:"type"` // text / image / tool_use / tool_result / thinking

	// text
	Text string `json:"text,omitempty"`

	// thinking
	Thinking string `json:"thinking,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID  string          `json:"tool_use_id,omitempty"`
	RawContent json.RawMessage `json:"-"` // 自定义序列化，不走默认 tag
	IsError    bool            `json:"is_error,omitempty"`
}

// MarshalJSON 按 Type 输出对应字段，避免多余字段导致 Anthropic 拒绝
func (b ContentBlock) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case "text":
		return json.Marshal(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{b.Type, b.Text})
	case "thinking":
		return json.Marshal(struct {
			Type     string `json:"type"`
			Thinking string `json:"thinking"`
		}{b.Type, b.Thinking})
	case "tool_use":
		return json.Marshal(struct {
			Type  string          `json:"type"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}{b.Type, b.ID, b.Name, b.Input})
	case "tool_result":
		v := struct {
			Type      string          `json:"type"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content,omitempty"`
			IsError   bool            `json:"is_error,omitempty"`
		}{b.Type, b.ToolUseID, b.RawContent, b.IsError}
		return json.Marshal(v)
	default:
		type Alias ContentBlock
		return json.Marshal((*Alias)(&b))
	}
}

// Tool 工具定义
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ---- 响应类型 ----

// MessagesResponse 非流式响应
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"` // message
	Role         string         `json:"role"` // assistant
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`   // end_turn / max_tokens / tool_use
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

// Usage token 用量
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ---- SSE 事件类型 ----

// SSEMessageStart message_start 事件
type SSEMessageStart struct {
	Type    string          `json:"type"`
	Message MessagesResponse `json:"message"`
}

// SSEContentBlockStart content_block_start 事件
type SSEContentBlockStart struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

// SSEContentBlockDelta content_block_delta 事件
type SSEContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta Delta  `json:"delta"`
}

// Delta 增量内容
type Delta struct {
	Type     string          `json:"type"` // text_delta / input_json_delta / thinking_delta
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	PartialJSON string       `json:"partial_json,omitempty"`
}

// SSEContentBlockStop content_block_stop 事件
type SSEContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// SSEMessageDelta message_delta 事件
type SSEMessageDelta struct {
	Type  string     `json:"type"`
	Delta MessageDelta `json:"delta"`
	Usage *Usage     `json:"usage,omitempty"`
}

// MessageDelta 消息级增量
type MessageDelta struct {
	StopReason   string  `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
}

// SSEError error 事件
type SSEError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
