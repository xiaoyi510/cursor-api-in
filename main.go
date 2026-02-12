package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"cursor-api-2-claude/internal/config"
	"cursor-api-2-claude/internal/handler"
	"cursor-api-2-claude/internal/middleware"

	"github.com/gin-gonic/gin"
)

//go:embed public
var publicFS embed.FS

func main() {
	if err := config.Load(); err != nil {
		log.Fatal("load config:", err)
	}

	publicSub, _ := fs.Sub(publicFS, "public")
	handler.PublicFS = publicSub
	staticFS, _ := fs.Sub(publicFS, "public/static")

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Static & admin page (no auth - login page must be accessible)
	r.GET("/admin", handler.AdminPage)
	r.StaticFS("/static", http.FS(staticFS))

	// Admin login (no auth)
	r.POST("/admin/api/login", handler.Login)
	r.GET("/admin/api/auth-check", handler.CheckAuth)

	// Admin API (admin password auth)
	adminAPI := r.Group("/admin/api", middleware.AdminAuth())
	{
		adminAPI.GET("/config", handler.GetConfig)
		adminAPI.PUT("/config", handler.PutConfig)
		adminAPI.POST("/providers/test", handler.TestProvider)
		adminAPI.POST("/providers/models", handler.FetchModels)
		adminAPI.POST("/providers/test-model", handler.TestModel)
	}

	// Proxy API (API key auth)
	v1 := r.Group("/v1", middleware.APIKeyAuth())
	{
		v1.POST("/chat/completions", handler.ChatCompletions)
		v1.POST("/messages", handler.Messages)
		v1.GET("/models", handler.Models)
	}

	r.GET("/health", handler.Health)

	c := config.Get()
	addr := fmt.Sprintf(":%d", c.Port)
	log.Printf("Server starting on %s", addr)
	log.Printf("Admin console: http://localhost:%d/admin", c.Port)
	r.Run(addr)
}
