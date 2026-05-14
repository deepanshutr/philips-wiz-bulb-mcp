package main

import (
	"fmt"
	"log"
	"os"

	"github.com/deepanshutr/philips-wiz-bulb-mcp/internal/core"
	"github.com/deepanshutr/philips-wiz-bulb-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetOutput(os.Stderr)

	if logPath := os.Getenv("PHILIPS_WIZ_BULB_MCP_LOG"); logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			log.SetOutput(f)
		}
	}

	coreURL := os.Getenv("PHILIPS_WIZ_BULB_CORE_URL")
	if coreURL == "" {
		coreURL = "http://127.0.0.1:8766"
	}
	c := core.New(coreURL)

	s := server.NewMCPServer("philips-wiz-bulb-mcp", "0.1.0",
		server.WithToolCapabilities(false),
	)
	tools.Register(s, c)

	log.Printf("philips-wiz-bulb-mcp starting; core=%s", coreURL)
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
