package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/adapter"
)

// Simulate an existing Gin API
func setupGinRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.GET("/api/v1/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, []map[string]any{
			{"id": "1", "name": "Alice"},
			{"id": "2", "name": "Bob"},
		})
	})

	r.GET("/api/v1/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"id": c.Param("id"), "name": "User " + c.Param("id"),
		})
	})

	r.POST("/api/v1/users", func(c *gin.Context) {
		var body map[string]any
		c.BindJSON(&body)
		body["id"] = "3"
		c.JSON(http.StatusCreated, body)
	})

	r.DELETE("/api/v1/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]any{"deleted": c.Param("id")})
	})

	return r
}

func main() {
	// 1. Your existing Gin router
	ginRouter := setupGinRouter()

	// 2. Create MCP server and import Gin routes — that's it!
	s := gomcp.New("gin-adapter-demo", "1.0.0")
	s.Use(gomcp.Logger())

	adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
		IncludePaths: []string{"/api/v1/"},
	})

	log.Println("Starting Gin adapter MCP server...")
	if err := s.Stdio(); err != nil {
		log.Fatal(err)
	}
}
