package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"cursor-api-2-claude/internal/adapter"
	"cursor-api-2-claude/internal/config"
	"cursor-api-2-claude/internal/proxy"

	"github.com/gin-gonic/gin"
)

func ChatCompletions(c *gin.Context) {
	cfg := config.Get()

	var req adapter.OAIRequest
	body, _ := io.ReadAll(c.Request.Body)
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("[400] invalid request body: %v, body: %s", err, string(body[:min(len(body), 200)]))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	targetModel, providers := proxy.ResolveModel(req.Model, cfg)
	if targetModel == "" || len(providers) == 0 {
		log.Printf("[400] no provider for model: %s", req.Model)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("no provider for model %s", req.Model)})
		return
	}

	provider := proxy.WeightedSelect(providers)
	log.Printf("[proxy] %s -> %s (provider: %s)", req.Model, targetModel, provider.ID)
	timeout := time.Duration(provider.Timeout) * time.Second
	if timeout == 0 {
		timeout = 300 * time.Second
	}

	switch provider.Type {
	case "anthropic":
		proxy.ProxyAnthropic(c.Writer, c.Request, req, provider, targetModel, timeout)
	default:
		proxy.ProxyOpenAI(c.Writer, c.Request, body, req, provider, targetModel, timeout)
	}
}

func Messages(c *gin.Context) {
	cfg := config.Get()

	body, _ := io.ReadAll(c.Request.Body)
	var raw struct {
		Model string `json:"model"`
	}
	json.Unmarshal(body, &raw)

	targetModel, providers := proxy.ResolveModel(raw.Model, cfg)
	if targetModel == "" || len(providers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("no provider for model %s", raw.Model)})
		return
	}

	provider := proxy.WeightedSelect(providers)

	var full map[string]json.RawMessage
	json.Unmarshal(body, &full)
	modelJSON, _ := json.Marshal(targetModel)
	full["model"] = modelJSON
	newBody, _ := json.Marshal(full)

	timeout := time.Duration(provider.Timeout) * time.Second
	if timeout == 0 {
		timeout = 300 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	url := strings.TrimRight(provider.BaseURL, "/") + "/v1/messages"
	httpReq, _ := http.NewRequestWithContext(c.Request.Context(), "POST", url, bytes.NewReader(newBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", provider.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	isStream := strings.Contains(resp.Header.Get("Content-Type"), "event-stream")
	if isStream {
		rc := http.NewResponseController(c.Writer)
		rc.SetWriteDeadline(time.Time{})
		buf := make([]byte, 32*1024)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				c.Writer.Write(buf[:n])
				c.Writer.Flush()
			}
			if err != nil {
				break
			}
		}
	} else {
		io.Copy(c.Writer, resp.Body)
	}
}

func Models(c *gin.Context) {
	cfg := config.Get()
	type model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}
	seen := map[string]bool{}
	var models []model
	for _, p := range cfg.Providers {
		for _, m := range p.Models {
			if m.Enabled && !seen[m.From] {
				seen[m.From] = true
				models = append(models, model{
					ID:      m.From,
					Object:  "model",
					Created: time.Now().Unix(),
					OwnedBy: "proxy",
				})
			}
		}
	}
	if models == nil {
		models = []model{}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": models})
}

func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
