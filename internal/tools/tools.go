// Package tools registers MCP tools against an mcp-go server.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deepanshutr/philips-wiz-bulb-mcp/internal/core"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const targetDesc = "Bulb selector: MAC (with or without colons), IPv4, friendly name, or 'all'. Omit to use the registry's default bulb (the earliest-discovered)."

func Register(s *server.MCPServer, c *core.Client) {
	s.AddTool(mcp.NewTool("philips_wiz_bulb_list",
		mcp.WithDescription("List all known Philips WiZ bulbs from the daemon's registry."),
	), wrap(func(ctx context.Context, _ mcp.CallToolRequest) (string, error) {
		out, err := c.List(ctx)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_state",
		mcp.WithDescription("Get a bulb's current state (on/off, brightness, color temp, scene)."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		out, err := c.Get(ctx, pick(req))
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_on",
		mcp.WithDescription("Turn a bulb on."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		if err := c.On(ctx, pick(req)); err != nil {
			return "", err
		}
		return "Bulb on.", nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_off",
		mcp.WithDescription("Turn a bulb off."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		if err := c.Off(ctx, pick(req)); err != nil {
			return "", err
		}
		return "Bulb off.", nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_brightness",
		mcp.WithDescription("Set bulb brightness, integer 10-100."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("level", mcp.Required(), mcp.Description("Brightness 10..100")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		level, err := req.RequireInt("level")
		if err != nil {
			return "", err
		}
		if err := c.Brightness(ctx, pick(req), level); err != nil {
			return "", err
		}
		return fmt.Sprintf("Brightness = %d.", level), nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_temp",
		mcp.WithDescription("Set bulb color temperature in Kelvin, 2200-6500."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("kelvin", mcp.Required(), mcp.Description("Color temp 2200..6500 K")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		k, err := req.RequireInt("kelvin")
		if err != nil {
			return "", err
		}
		if err := c.Temp(ctx, pick(req), k); err != nil {
			return "", err
		}
		return fmt.Sprintf("Temp = %dK.", k), nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_color",
		mcp.WithDescription("Set bulb color via RGB (each 0-255)."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("r", mcp.Required(), mcp.Description("Red 0..255")),
		mcp.WithNumber("g", mcp.Required(), mcp.Description("Green 0..255")),
		mcp.WithNumber("b", mcp.Required(), mcp.Description("Blue 0..255")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		r, err := req.RequireInt("r")
		if err != nil {
			return "", err
		}
		g, err := req.RequireInt("g")
		if err != nil {
			return "", err
		}
		b, err := req.RequireInt("b")
		if err != nil {
			return "", err
		}
		if err := c.Color(ctx, pick(req), r, g, b); err != nil {
			return "", err
		}
		return fmt.Sprintf("Color = rgb(%d,%d,%d).", r, g, b), nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_scene",
		mcp.WithDescription("Set a built-in WiZ scene by name (e.g. 'cozy', 'ocean', 'sunset', 'warm white', 'night light') or numeric id 1-32."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithString("scene", mcp.Required(), mcp.Description("Scene name or id 1..32")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		scene, err := req.RequireString("scene")
		if err != nil {
			return "", err
		}
		if err := c.Scene(ctx, pick(req), scene); err != nil {
			return "", err
		}
		return "Scene = " + scene + ".", nil
	}))

	s.AddTool(mcp.NewTool("philips_wiz_bulb_discover",
		mcp.WithDescription("Force a LAN rescan for new bulbs."),
	), wrap(func(ctx context.Context, _ mcp.CallToolRequest) (string, error) {
		out, err := c.Discover(ctx)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), nil
	}))
}

func pick(req mcp.CallToolRequest) string {
	t := req.GetString("target", "")
	if t == "" {
		return "_default"
	}
	return t
}

func wrap(fn func(context.Context, mcp.CallToolRequest) (string, error)) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, err := fn(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	}
}
