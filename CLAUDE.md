# CLAUDE.md — philips-wiz-bulb-mcp

MCP stdio server talking HTTP to philips-wiz-bulb-core.

## Conventions

- Go 1.23, library: `github.com/mark3labs/mcp-go`
- `unset GOROOT; export GOPROXY=https://proxy.golang.org,direct` before any `go` command
- Per-repo: `git config user.email 52166434+deepanshutr@users.noreply.github.com`
- `log.SetOutput(os.Stderr)` is mandatory — MCP protocol uses stdout, any
  stray log line to stdout corrupts the wire format
- Run `go vet`, `go test ./...`, `./scripts/smoke.sh ./philips-wiz-bulb-mcp` before commit

## Layout

```
cmd/philips-wiz-bulb-mcp/main.go
internal/core/client.go     # HTTP client (self-contained copy of philips-wiz-bulb-cli's)
internal/tools/tools.go     # 9 MCP tool registrations
scripts/smoke.sh            # stdio JSON-RPC initialize + tools/list
```

## Register with Claude Code

```bash
claude mcp add philips-wiz-bulb ~/.local/bin/philips-wiz-bulb-mcp -s user
```

A running Claude session must be restarted to pick up new tool schemas — MCP
reads them at initialize-time.
