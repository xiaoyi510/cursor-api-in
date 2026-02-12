package adapter

import (
	"encoding/json"
	"fmt"
	"time"
)

func ContentToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var result string
		for _, b := range blocks {
			if b.Type == "text" {
				result += b.Text
			}
		}
		return result
	}
	return string(raw)
}

func OpenaiToAnthropic(req OAIRequest, model string) AnthropicRequest {
	ar := AnthropicRequest{
		Model:       model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		ar.MaxTokens = *req.MaxTokens
	}
	if ar.MaxTokens < 8192 {
		ar.MaxTokens = 8192
	}

	for _, t := range req.Tools {
		ar.Tools = append(ar.Tools, AnthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	if len(req.ToolChoice) > 0 && len(ar.Tools) > 0 {
		var s string
		if json.Unmarshal(req.ToolChoice, &s) == nil {
			switch s {
			case "auto":
				ar.ToolChoice, _ = json.Marshal(map[string]string{"type": "auto"})
			case "required":
				ar.ToolChoice, _ = json.Marshal(map[string]string{"type": "any"})
			case "none":
				ar.Tools = nil
			}
		} else {
			var obj struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if json.Unmarshal(req.ToolChoice, &obj) == nil && obj.Function.Name != "" {
				ar.ToolChoice, _ = json.Marshal(map[string]string{"type": "tool", "name": obj.Function.Name})
			}
		}
	}

	var msgs []AnthropicMsg
	for _, m := range req.Messages {
		if m.Role == "system" {
			ar.System += ContentToString(m.Content)
			continue
		}

		role := m.Role
		var blocks []ContentBlock

		if m.Role == "tool" {
			role = "user"
			blocks = append(blocks, ContentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			})
		} else if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			text := ContentToString(m.Content)
			if text != "" {
				blocks = append(blocks, ContentBlock{Type: "text", Text: text})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: json.RawMessage(tc.Function.Arguments),
				})
			}
		} else {
			text := ContentToString(m.Content)
			if text != "" {
				blocks = append(blocks, ContentBlock{Type: "text", Text: text})
			}
		}

		if len(blocks) == 0 {
			blocks = append(blocks, ContentBlock{Type: "text", Text: " "})
		}

		blockJSON, _ := json.Marshal(blocks)

		if len(msgs) > 0 && msgs[len(msgs)-1].Role == role {
			var prev []ContentBlock
			json.Unmarshal(msgs[len(msgs)-1].Content, &prev)
			prev = append(prev, blocks...)
			merged, _ := json.Marshal(prev)
			msgs[len(msgs)-1].Content = merged
		} else {
			msgs = append(msgs, AnthropicMsg{Role: role, Content: blockJSON})
		}
	}

	ar.Messages = msgs
	return ar
}

func MapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return "stop"
	}
}

func AnthropicToOpenai(resp AnthropicResponse, model string) OAIResponse {
	msg := OAIMsg{Role: "assistant"}
	toolIdx := 0
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			msg.Content += block.Text
		case "thinking":
			msg.ReasoningContent += block.Thinking
		case "tool_use":
			inputStr, _ := json.Marshal(block.Input)
			msg.ToolCalls = append(msg.ToolCalls, OAIToolCall{
				Index: toolIdx,
				ID:    block.ID,
				Type:  "function",
				Function: OAIFunctionCall{
					Name:      block.Name,
					Arguments: string(inputStr),
				},
			})
			toolIdx++
		}
	}

	fr := MapStopReason(resp.StopReason)

	oaiResp := OAIResponse{
		ID:      "chatcmpl-" + resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []OAIChoice{{Index: 0, Message: &msg, FinishReason: &fr}},
	}
	if resp.Usage != nil {
		oaiResp.Usage = &OAIUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	return oaiResp
}

func AnthropicStreamEventToChunks(eventType string, data json.RawMessage, state *StreamState, model string) []OAIResponse {
	var chunks []OAIResponse
	makeChunk := func(delta OAIMsg, finish *string) OAIResponse {
		return OAIResponse{
			ID:      "chatcmpl-stream",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []OAIChoice{{Index: 0, Delta: &delta, FinishReason: finish}},
		}
	}

	switch eventType {
	case "message_start":
		chunks = append(chunks, makeChunk(OAIMsg{Role: "assistant"}, nil))

	case "content_block_start":
		var block struct {
			Index        int          `json:"index"`
			ContentBlock ContentBlock `json:"content_block"`
		}
		json.Unmarshal(data, &block)
		if block.ContentBlock.Type == "tool_use" {
			state.HasTool = true
			state.ToolID = block.ContentBlock.ID
			state.ToolName = block.ContentBlock.Name
			state.ToolArgs = ""
			tc := OAIToolCall{
				Index:    state.ToolIndex,
				ID:       state.ToolID,
				Type:     "function",
				Function: OAIFunctionCall{Name: state.ToolName, Arguments: ""},
			}
			delta := OAIMsg{ToolCalls: []OAIToolCall{tc}}
			chunks = append(chunks, makeChunk(delta, nil))
			state.ToolIndex++
		}

	case "content_block_delta":
		var d struct {
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		json.Unmarshal(data, &d)
		switch d.Delta.Type {
		case "text_delta":
			chunks = append(chunks, makeChunk(OAIMsg{Content: d.Delta.Text}, nil))
		case "thinking_delta":
			chunks = append(chunks, makeChunk(OAIMsg{ReasoningContent: d.Delta.Thinking}, nil))
		case "input_json_delta":
			state.ToolArgs += d.Delta.PartialJSON
			tc := OAIToolCall{
				Index:    state.ToolIndex - 1,
				ID:       state.ToolID,
				Type:     "function",
				Function: OAIFunctionCall{Name: state.ToolName, Arguments: d.Delta.PartialJSON},
			}
			chunks = append(chunks, makeChunk(OAIMsg{ToolCalls: []OAIToolCall{tc}}, nil))
		}

	case "message_delta":
		var d struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage *AnthropicUsage `json:"usage"`
		}
		json.Unmarshal(data, &d)
		fr := MapStopReason(d.Delta.StopReason)
		chunk := makeChunk(OAIMsg{}, &fr)
		if d.Usage != nil {
			chunk.Usage = &OAIUsage{
				PromptTokens:     d.Usage.InputTokens,
				CompletionTokens: d.Usage.OutputTokens,
				TotalTokens:      d.Usage.InputTokens + d.Usage.OutputTokens,
			}
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

func FormatSSEChunk(chunk OAIResponse) string {
	data, _ := json.Marshal(chunk)
	return fmt.Sprintf("data: %s\n\n", data)
}
