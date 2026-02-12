package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"cursor-api-2-claude/internal/config"

	"github.com/gin-gonic/gin"
)

var (
	tokens   = map[string]time.Time{}
	tokensMu sync.RWMutex
)

func GenerateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	tokensMu.Lock()
	tokens[token] = time.Now().Add(24 * time.Hour)
	tokensMu.Unlock()
	return token
}

func ValidateToken(token string) bool {
	tokensMu.RLock()
	exp, ok := tokens[token]
	tokensMu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		tokensMu.Lock()
		delete(tokens, token)
		tokensMu.Unlock()
		return false
	}
	return true
}

func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()
		if cfg.APIKey == "" {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		xKey := c.GetHeader("x-api-key")
		if auth != "Bearer "+cfg.APIKey && xKey != cfg.APIKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()
		if cfg.AdminPassword == "" {
			c.Next()
			return
		}
		token, err := c.Cookie("admin_token")
		if err != nil || !ValidateToken(token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
