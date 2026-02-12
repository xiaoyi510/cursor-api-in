package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"time"

	"cursor-api-2-claude/internal/config"
	"cursor-api-2-claude/internal/middleware"

	"github.com/gin-gonic/gin"
)

var PublicFS fs.FS // set from main.go

var anthropicModels = []string{
	"claude-opus-4-6",
	"claude-sonnet-4-5",
	"claude-haiku-4-5",
	"claude-opus-4-20250514",
	"claude-sonnet-4-20250514",
	"claude-sonnet-4-5-20250929",
	"claude-haiku-4-5-20251001",
	"claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-20241022",
	"claude-3-opus-20240229",
	"claude-3-haiku-20240307",
}

func AdminPage(c *gin.Context) {
	data, _ := fs.ReadFile(PublicFS, "admin.html")
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func Login(c *gin.Context) {
	cfg := config.Get()
	if cfg.AdminPassword == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid request"})
		return
	}
	if req.Password != cfg.AdminPassword {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "密码错误"})
		return
	}
	token := middleware.GenerateToken()
	c.SetCookie("admin_token", token, 86400, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": token})
}

func CheckAuth(c *gin.Context) {
	cfg := config.Get()
	if cfg.AdminPassword == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "need_login": false})
		return
	}
	token, err := c.Cookie("admin_token")
	if err != nil || !middleware.ValidateToken(token) {
		c.JSON(http.StatusOK, gin.H{"ok": false, "need_login": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "need_login": false})
}

func GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, config.Get())
}

func PutConfig(c *gin.Context) {
	var cfg config.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if err := config.Set(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, config.Get())
}

func TestProvider(c *gin.Context) {
	var p config.Provider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var req *http.Request

	switch p.Type {
	case "anthropic":
		body, _ := json.Marshal(map[string]any{
			"model":      "claude-sonnet-4-5-20250929",
			"max_tokens": 1,
			"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		})
		req, _ = http.NewRequest("POST", p.BaseURL+"/v1/messages", bytes.NewReader(body))
		req.Header.Set("x-api-key", p.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")
	default:
		req, _ = http.NewRequest("GET", p.BaseURL+"/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	c.JSON(http.StatusOK, gin.H{
		"ok":     resp.StatusCode >= 200 && resp.StatusCode < 500,
		"status": resp.StatusCode,
		"body":   string(respBody),
	})
}

func FetchModels(c *gin.Context) {
	var p config.Provider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if p.Type == "anthropic" {
		c.JSON(http.StatusOK, gin.H{"models": anthropicModels})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", p.BaseURL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "failed to parse response"})
		return
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func TestModel(c *gin.Context) {
	var req struct {
		Provider config.Provider `json:"provider"`
		Model    string          `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid json"})
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	start := time.Now()
	var httpReq *http.Request

	switch req.Provider.Type {
	case "anthropic":
		body, _ := json.Marshal(map[string]any{
			"model":      req.Model,
			"max_tokens": 1,
			"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
		})
		httpReq, _ = http.NewRequest("POST", req.Provider.BaseURL+"/v1/messages", bytes.NewReader(body))
		httpReq.Header.Set("x-api-key", req.Provider.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		httpReq.Header.Set("Content-Type", "application/json")
	default:
		body, _ := json.Marshal(map[string]any{
			"model":      req.Model,
			"max_tokens": 1,
			"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
		})
		httpReq, _ = http.NewRequest("POST", req.Provider.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
		httpReq.Header.Set("Authorization", "Bearer "+req.Provider.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error(), "latency_ms": latency})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	ok := resp.StatusCode >= 200 && resp.StatusCode < 300

	// Try to extract reply text
	var reply string
	if req.Provider.Type == "anthropic" {
		var ar struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(respBody, &ar) == nil && len(ar.Content) > 0 {
			reply = ar.Content[0].Text
		}
	} else {
		var or struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(respBody, &or) == nil && len(or.Choices) > 0 {
			reply = or.Choices[0].Message.Content
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":         ok,
		"status":     resp.StatusCode,
		"latency_ms": latency,
		"reply":      reply,
		"error":      func() string { if !ok { return string(respBody) }; return "" }(),
	})
}
