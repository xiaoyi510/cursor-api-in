package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"path"
	"strings"
	"time"

	"cursor-api-2-claude/internal/adapter"
	"cursor-api-2-claude/internal/config"
)

func ResolveModel(model string, c config.Config) (string, []config.Provider) {
	type match struct {
		provider config.Provider
		to       string
	}
	var matches []match
	for _, p := range c.Providers {
		if p.Weight <= 0 {
			continue
		}
		for _, m := range p.Models {
			if m.Enabled && matchModel(m.From, model) {
				matches = append(matches, match{provider: p, to: m.To})
				break
			}
		}
	}
	if len(matches) == 0 {
		return "", nil
	}
	targetModel := matches[0].to
	var providers []config.Provider
	for _, m := range matches {
		providers = append(providers, m.provider)
	}
	return targetModel, providers
}

func matchModel(pattern, model string) bool {
	if pattern == "*" {
		return true
	}
	matched, _ := path.Match(pattern, model)
	return matched
}

func WeightedSelect(providers []config.Provider) config.Provider {
	total := 0
	for _, p := range providers {
		total += p.Weight
	}
	r := rand.IntN(total)
	for _, p := range providers {
		r -= p.Weight
		if r < 0 {
			return p
		}
	}
	return providers[0]
}

func ProxyAnthropic(w http.ResponseWriter, r *http.Request, req adapter.OAIRequest, p config.Provider, model string, timeout time.Duration) {
	ar := adapter.OpenaiToAnthropic(req, model)
	arBody, _ := json.Marshal(ar)

	log.Printf("[DEBUG] ===== Anthropic Request =====\n%s", indentJSON(arBody))

	client := &http.Client{Timeout: timeout}
	url := strings.TrimRight(p.BaseURL, "/") + "/v1/messages"
	httpReq, _ := http.NewRequestWithContext(r.Context(), "POST", url, bytes.NewReader(arBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[DEBUG] Anthropic request error: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	log.Printf("[DEBUG] Anthropic response status: %d", resp.StatusCode)

	if resp.StatusCode != 200 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[DEBUG] ===== Anthropic Error Response =====\n%s", string(respBody))
		w.Write(respBody)
		return
	}

	if req.Stream {
		StreamAnthropicToOpenAI(w, resp.Body, req.Model)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[DEBUG] ===== Anthropic Response =====\n%s", indentJSON(respBody))
		var ar adapter.AnthropicResponse
		if err := json.Unmarshal(respBody, &ar); err != nil {
			http.Error(w, `{"error":"decode error"}`, http.StatusBadGateway)
			return
		}
		oai := adapter.AnthropicToOpenai(ar, req.Model)
		oaiBody, _ := json.Marshal(oai)
		log.Printf("[DEBUG] ===== OAI Response =====\n%s", indentJSON(oaiBody))
		w.Header().Set("Content-Type", "application/json")
		w.Write(oaiBody)
	}
}

func ProxyAnthropicRaw(w http.ResponseWriter, r *http.Request, body []byte, p config.Provider, originalModel string, timeout time.Duration) {
	client := &http.Client{Timeout: timeout}
	url := strings.TrimRight(p.BaseURL, "/") + "/v1/messages"
	httpReq, _ := http.NewRequestWithContext(r.Context(), "POST", url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[DEBUG] Anthropic request error: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	log.Printf("[DEBUG] Anthropic response status: %d", resp.StatusCode)

	if resp.StatusCode != 200 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[DEBUG] ===== Anthropic Error Response =====\n%s", string(respBody))
		w.Write(respBody)
		return
	}

	// 响应需要转换为 OpenAI 格式，因为请求来自 /v1/chat/completions
	isStream := strings.Contains(resp.Header.Get("Content-Type"), "event-stream")
	if isStream {
		StreamAnthropicToOpenAI(w, resp.Body, originalModel)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[DEBUG] ===== Anthropic Raw Response =====\n%s", string(respBody))
		var ar adapter.AnthropicResponse
		if err := json.Unmarshal(respBody, &ar); err != nil {
			http.Error(w, `{"error":"decode error"}`, http.StatusBadGateway)
			return
		}
		oai := adapter.AnthropicToOpenai(ar, originalModel)
		oaiBody, _ := json.Marshal(oai)
		w.Header().Set("Content-Type", "application/json")
		w.Write(oaiBody)
	}
}

func StreamAnthropicToOpenAI(w http.ResponseWriter, body io.Reader, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	state := &adapter.StreamState{}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var currentEvent string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		log.Printf("[DEBUG] [SSE] event=%s data=%s", currentEvent, data)
		chunks := adapter.AnthropicStreamEventToChunks(currentEvent, json.RawMessage(data), state, model)
		for _, chunk := range chunks {
			sse := adapter.FormatSSEChunk(chunk)
			log.Printf("[DEBUG] [SSE->OAI] %s", strings.TrimSpace(sse))
			fmt.Fprint(w, sse)
			flusher.Flush()
		}
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func indentJSON(data []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") == nil {
		return buf.String()
	}
	return string(data)
}

func ProxyOpenAI(w http.ResponseWriter, r *http.Request, body []byte, req adapter.OAIRequest, p config.Provider, model string, timeout time.Duration) {
	var raw map[string]json.RawMessage
	json.Unmarshal(body, &raw)
	modelJSON, _ := json.Marshal(model)
	raw["model"] = modelJSON
	newBody, _ := json.Marshal(raw)

	client := &http.Client{Timeout: timeout}
	url := strings.TrimRight(p.BaseURL, "/") + "/v1/chat/completions"
	httpReq, _ := http.NewRequestWithContext(r.Context(), "POST", url, bytes.NewReader(newBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if req.Stream {
		rc := http.NewResponseController(w)
		rc.SetWriteDeadline(time.Time{})
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 32*1024)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil {
				break
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}
}
