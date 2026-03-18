// CronControl MCP Server binary.
//
// Runs as an MCP server (stdin/stdout JSON-RPC) for AI agent integration.
//
// Usage:
//   CRONCONTROL_URL=https://croncontrol.example.com CRONCONTROL_API_KEY=cc_live_... croncontrol-mcp
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/croncontrol/croncontrol/mcp"
)

func main() {
	url := os.Getenv("CRONCONTROL_URL")
	if url == "" {
		url = "http://localhost:8080"
	}

	apiKey := os.Getenv("CRONCONTROL_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "CRONCONTROL_API_KEY is required")
		os.Exit(1)
	}

	// Log to stderr (stdout is for MCP protocol)
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	server := mcp.NewServer(url, apiKey)
	if err := server.Run(); err != nil {
		slog.Error("mcp server error", "error", err)
		os.Exit(1)
	}
}
