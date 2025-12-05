package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DEFAULT_PORT    = "9621"
	DEFAULT_TIMEOUT = 30 * time.Second
)

func main() {
	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = DEFAULT_PORT
	}

	// 创建 SSE Handler
	sseHandler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		server := mcp.NewServer(&mcp.Implementation{
			Name:    "web-browser",
			Version: "1.0.0",
		}, nil)

		// 注册工具
		registerTools(server)

		return server
	}, nil)

	// 启动 HTTP 服务器
	addr := ":" + port
	log.Printf("🌐 Web Browser MCP Server 启动中...")
	log.Printf("📡 SSE 端点: http://localhost%s/", addr)
	log.Printf("📨 使用官方 go-sdk 的 SSE Transport")

	if err := http.ListenAndServe(addr, sseHandler); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
