# philips-wiz-bulb-mcp

MCP stdio server exposing 9 Philips WiZ bulb-control tools to Claude Code.
Backed by [philips-wiz-bulb-core](https://github.com/deepanshutr/philips-wiz-bulb-core).

| Tool | Params |
|---|---|
| `philips_wiz_bulb_list` | — |
| `philips_wiz_bulb_state` | `target?` |
| `philips_wiz_bulb_on` / `_off` | `target?` |
| `philips_wiz_bulb_brightness` | `target?`, `level: 10..100` |
| `philips_wiz_bulb_temp` | `target?`, `kelvin: 2200..6500` |
| `philips_wiz_bulb_color` | `target?`, `r,g,b: 0..255` |
| `philips_wiz_bulb_scene` | `target?`, `scene: name\|id`, `speed?` |
| `philips_wiz_bulb_discover` | — |

`target` accepts MAC, IPv4, friendly name, or `all`. Omit to use the
registry's default bulb (earliest discovered).

## Register with Claude Code

```bash
claude mcp add philips-wiz-bulb ~/.local/bin/philips-wiz-bulb-mcp -s user
```

A running Claude session must be restarted to pick up new tool schemas.

See [`docs/superpowers/specs/2026-05-14-philips-wiz-bulb-stack-design.md`](docs/superpowers/specs/2026-05-14-philips-wiz-bulb-stack-design.md)
for the full design.

## Sibling repos

- [philips-wiz-bulb-core](https://github.com/deepanshutr/philips-wiz-bulb-core) — Python daemon
- [philips-wiz-bulb-cli](https://github.com/deepanshutr/philips-wiz-bulb-cli) — Go cobra CLI
