package adapter

import "encoding/json"

// --- OpenAI Types ---

type OAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OAIMessage    `json:"messages"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
	Tools       []OAITool       `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
	Stop        json.RawMessage `json:"stop,omitempty"`
	System      json.RawMessage `json:"system,omitempty"`
}

type OAIMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Name       string          `json:"name,omitempty"`
	ToolCalls  []OAIToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type OAIToolCall struct {
	Index    int             `json:"index"`
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type"`
	Function OAIFunctionCall `json:"function"`
}

type OAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OAITool struct {
	Type     string      `json:"type"`
	Function OAIFunction `json:"function"`
}

func (t *OAITool) UnmarshalJSON(data []byte) error {
	// 先尝试标准 OpenAI 格式: {"type":"function","function":{...}}
	type plain OAITool
	if err := json.Unmarshal(data, (*plain)(t)); err == nil && t.Function.Name != "" {
		return nil
	}
	// Cursor 扁平格式: {"name":"...","description":"...","input_schema":{...}}
	var flat struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	if err := json.Unmarshal(data, &flat); err != nil {
		return err
	}
	t.Type = "function"
	t.Function = OAIFunction{
		Name:        flat.Name,
		Description: flat.Description,
		Parameters:  flat.InputSchema,
	}
	return nil
}

type OAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type OAIResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []OAIChoice `json:"choices"`
	Usage   *OAIUsage   `json:"usage,omitempty"`
}

type OAIChoice struct {
	Index        int     `json:"index"`
	Message      *OAIMsg `json:"message,omitempty"`
	Delta        *OAIMsg `json:"delta,omitempty"`
	FinishReason *string `json:"finish_reason"`
}

type OAIMsg struct {
	Role             string        `json:"role,omitempty"`
	Content          string        `json:"content,omitempty"`
	ToolCalls        []OAIToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
}

type OAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Anthropic Types ---

type AnthropicRequest struct {
	Model       string          `json:"model"`
	Messages    []AnthropicMsg  `json:"messages"`
	System      string          `json:"system,omitempty"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
	Tools       []AnthropicTool `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
}

type AnthropicMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type AnthropicResponse struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Content    []ContentBlock  `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	Usage      *AnthropicUsage `json:"usage,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// --- Stream State ---

type StreamState struct {
	ToolIndex int
	ToolID    string
	ToolName  string
	ToolArgs  string
	HasTool   bool
}
