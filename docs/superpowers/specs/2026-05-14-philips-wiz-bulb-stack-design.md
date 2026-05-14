# Philips WiZ bulb stack — design

**Status:** approved 2026-05-14
**Owner:** deepanshutr
**Models:** mirrors the `lgtv` 3-repo stack pattern shipped 2026-05-11.

## Goal

Autonomous control of the Philips WiZ smart bulb(s) on the LAN, with the same
3-repo shape as the LG-TV stack so the existing muscle memory (binaries,
systemd units, env-var prefix, MCP registration, orchctl-v2 slash command)
carries over without re-learning.

As of design time: one bulb discovered on the LAN.

| Field | Value |
|---|---|
| IP | `192.168.1.3` (DHCP — must not be hard-coded) |
| MAC | `d8:a0:11:8d:c5:c3` (OUI `d8:a0:11` = Signify Netherlands B.V.) |
| Product line | Philips WiZ (Wi-Fi, no bridge) |
| Module | `ESP01_SHRGB1C_31` — ESP8266, single head, RGB + tunable white |
| CCT range | 2200 K – 6500 K |
| Firmware | 1.35.0 (EU region) |
| Protocol | JSON over UDP **38899** (`getPilot` / `setPilot` / `getSystemConfig`) |

The design must scale to N bulbs without API breaks since WiZ commonly ships
in multi-packs.

## Non-goals

- Cloud integration (the WiZ app's `homeId`/`roomId` cloud binding is left
  alone; daemon drives bulbs purely on the LAN).
- Adding non-WiZ bulb stacks (Hue, Tuya, Tapo). The `philips-wiz-bulb-`
  prefix keeps a future Hue stack possible alongside without name collision,
  but Hue's bridge + HTTPS API has nothing in common with WiZ's UDP/JSON
  protocol — a unified daemon would be a misguided abstraction.
- Standalone Telegram bot in `philips-wiz-bulb-cli` — Telegram lives in
  orchctl-v2 (mirroring the post-cutover state where `lgtv-cli tg-bot`
  exists but isn't running).

## Architecture

```
Claude   ─MCP/stdio─▶  philips-wiz-bulb-mcp ─HTTP─▶  philips-wiz-bulb-core ─UDP:38899─▶ WiZ bulb(s)
Telegram ─long-poll─▶  orchctl-v2 (/bulb)    ─HTTP─▶ philips-wiz-bulb-core ───┘
Shell    ─exec─▶       philips-wiz-bulb (CLI) ─HTTP─▶ philips-wiz-bulb-core ──┘
```

| Repo | Lang | Role | Daemon port |
|---|---|---|---|
| `philips-wiz-bulb-core` | Python 3.11 / FastAPI | HTTP daemon | **127.0.0.1:8766** (8765 is lgtv-core) |
| `philips-wiz-bulb-cli` | Go 1.23 (cobra) | CLI | — |
| `philips-wiz-bulb-mcp` | Go 1.23 (mark3labs/mcp-go) | MCP stdio, 9 tools | — |

## 1. `philips-wiz-bulb-core` (Python daemon)

### Package layout

```
philips_wiz_bulb_core/
  __init__.py
  main.py        # FastAPI app + lifespan: boot discovery + start refresh loop
  cli.py         # uvicorn entrypoint (console_script: philips-wiz-bulb-core)
  config.py      # Pydantic Settings from PHILIPS_WIZ_BULB_* env
  bulb.py        # asyncio UDP client: getPilot / setPilot / getSystemConfig
  discover.py    # broadcast on $BROADCAST:38899 + unicast sweep of $SUBNET
  registry.py    # bulb registry keyed by MAC; state.json persistence
  api.py         # FastAPI routes
  scenes.py      # 32 WiZ scene constants (id ↔ canonical lowercase name)
```

### HTTP endpoints (all `127.0.0.1:8766`)

| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/health` | — | liveness |
| GET | `/bulbs` | — | full registry |
| POST | `/discover` | `{passive?: bool}` | re-scan LAN; `passive=true` only refreshes existing entries |
| GET | `/bulb/{target}` | — | resolve + `getPilot` |
| POST | `/bulb/{target}/on` | — | `setPilot state=true` |
| POST | `/bulb/{target}/off` | — | `setPilot state=false` |
| POST | `/bulb/{target}/brightness` | `{level: 10..100}` | `dimming` |
| POST | `/bulb/{target}/temp` | `{kelvin: 2200..6500}` | `temp` |
| POST | `/bulb/{target}/color` | `{r,g,b: 0..255}` | RGB |
| POST | `/bulb/{target}/scene` | `{scene: name\|id, speed?: 10..200}` | `sceneId` |
| POST | `/bulb/{target}/name` | `{name: string}` | edit friendly name |
| GET | `/scenes` | — | list 32 scenes |

### Target resolution rules (`registry.py`)

In order:

1. `target` omitted / empty → the bulb with the **earliest `discovered_at`**
   (deterministic across restarts). If the registry is empty, 409
   `{error: "no bulbs known; POST /discover first"}`.
2. `target == "all"` → broadcast op to every known bulb; response is a per-MAC result map.
3. `target` looks like MAC (with or without colons, case-insensitive) → exact MAC match.
4. `target` is a valid IPv4 → exact `last_ip` match.
5. else → case-insensitive friendly-name match.
6. otherwise 404 `{error: "no bulb matches <target>"}`.

### Registry persistence

`~/.config/philips-wiz-bulb/state.json` (mode 0600, gitignored):

```json
{
  "version": 1,
  "bulbs": {
    "d8a0118dc5c3": {
      "name": "bulb-1",
      "last_ip": "192.168.1.3",
      "last_rssi": -63,
      "module": "ESP01_SHRGB1C_31",
      "fw_version": "1.35.0",
      "cct_range": [2200, 6500],
      "discovered_at": "2026-05-14T16:00:00Z",
      "last_seen":    "2026-05-14T16:05:00Z"
    }
  }
}
```

Friendly name is auto-assigned as `bulb-N` on first discovery and is the only
mutable field via `/bulb/{target}/name`.

### Discovery and refresh loop

- **Boot:** load `state.json`, then full discover —
  broadcast `getPilot` to `$BROADCAST:38899` (3 s collect window) **then**
  unicast `getPilot` sweep of `$SUBNET` (parallel, 1.5 s per host) to catch
  AP-isolation cases.
- **Background:** every `REFRESH_INTERVAL_S` (default 60 s) unicast-ping
  known bulbs to refresh `last_seen` / `last_rssi` and detect IP drift.
  Every `DISCOVER_INTERVAL_S` (default 600 s) run a full re-discover to
  pick up new bulbs.
- **Reactive recovery:** on `setPilot` UDP timeout or `EHOSTUNREACH`,
  trigger an immediate re-discover for that MAC (broadcast + sweep), then
  retry the operation **once** before bubbling 504.

### WiZ UDP client (`bulb.py`)

- `asyncio.DatagramProtocol`-based; one transient socket per request.
- 1.5 s timeout per attempt, 2 retries with exponential backoff
  (1.5 s → 3 s).
- Surfaces WiZ error envelopes (`{"error": {"code": ..., "message": "..."}}`)
  as 502 with the error payload.

### Config (env-driven, all prefixed `PHILIPS_WIZ_BULB_`)

| Var | Default | Notes |
|---|---|---|
| `BIND` | `127.0.0.1:8766` | uvicorn bind |
| `BROADCAST` | `192.168.1.255` | UDP broadcast target |
| `SUBNET` | `192.168.1.0/24` | unicast-sweep range |
| `DISCOVER_INTERVAL_S` | `600` | full re-discover cadence |
| `REFRESH_INTERVAL_S` | `60` | per-bulb refresh cadence |
| `LOG_LEVEL` | `info` | |
| `STATE_DIR` | `~/.config/philips-wiz-bulb` | state.json location |

Shipped in `.env.example`.

### Built-in scenes (`scenes.py`)

Hardcoded const table mapping the 32 WiZ scene ids to canonical lowercase
names (`ocean`, `romance`, `sunset`, `party`, `fireplace`, `cozy`, `forest`,
`pastel colors`, `wake up`, `bedtime`, `warm white`, `daylight`,
`cool white`, `night light`, `focus`, `relax`, `true colors`, `tv time`,
`plantgrowth`, `spring`, `summer`, `fall`, `deepdive`, `jungle`, `mojito`,
`club`, `christmas`, `halloween`, `candlelight`, `golden white`, `pulse`,
`steampunk`). `/scene` accepts name OR numeric id; name lookup is
case-insensitive.

### Tests

| File | Coverage |
|---|---|
| `tests/test_registry.py` | resolution rules (all/mac/ip/name/404), state.json roundtrip, friendly-name autogen |
| `tests/test_bulb.py` | UDP client via mocked `DatagramProtocol`: getPilot/setPilot happy + timeout + WiZ error envelope |
| `tests/test_discover.py` | broadcast + unicast sweep against a fake socket harness |
| `tests/test_api.py` | FastAPI `TestClient` end-to-end, registry seeded in fixture |

### Systemd

`systemd/philips-wiz-bulb-core.service` — user unit, identical structure to
`lgtv-core.service` (NoNewPrivileges / PrivateTmp / ProtectSystem only —
the heavier `Protect*` knobs trip status=218 in user scope per
`feedback_systemd_user_hardening`). Restart with
`systemctl --user restart philips-wiz-bulb-core`, logs via
`journalctl --user -u philips-wiz-bulb-core -f`.

## 2. `philips-wiz-bulb-cli` (Go)

### Layout

```
cmd/philips-wiz-bulb/main.go
internal/
  cli/root.go          # cobra
  core/client.go       # HTTP client → daemon
  core/client_test.go
  config/config.go     # env: PHILIPS_WIZ_BULB_CORE_URL
```

### Subcommands

All accept optional positional `target` (default = first bulb in registry):

```
philips-wiz-bulb list
philips-wiz-bulb state    [target]
philips-wiz-bulb on       [target]
philips-wiz-bulb off      [target]
philips-wiz-bulb bri      <0-100>           [target]
philips-wiz-bulb temp     <2200-6500>       [target]
philips-wiz-bulb color    <r> <g> <b>       [target]
philips-wiz-bulb scene    <name|id>         [target]
philips-wiz-bulb discover
philips-wiz-bulb name     <mac|ip>          <new-name>
```

Binary installed at `~/.local/bin/philips-wiz-bulb` via `go install`.

## 3. `philips-wiz-bulb-mcp` (Go)

### Layout

```
cmd/philips-wiz-bulb-mcp/main.go
internal/
  core/client.go       # HTTP client (same shape as cli's; small enough that a copy is fine)
  core/client_test.go
  tools/tools.go       # 9 MCP tool definitions
scripts/smoke.sh       # stdio JSON-RPC initialize + tools/list (per feedback_mcp_stdio_smoke_test)
```

### Tools (9)

All `target` params are optional. When omitted, the daemon resolves to the
first bulb in the registry.

| Tool | Params |
|---|---|
| `philips_wiz_bulb_list` | — |
| `philips_wiz_bulb_state` | `target?` |
| `philips_wiz_bulb_on` | `target?` |
| `philips_wiz_bulb_off` | `target?` |
| `philips_wiz_bulb_brightness` | `target?`, `level: 10..100` |
| `philips_wiz_bulb_temp` | `target?`, `kelvin: 2200..6500` |
| `philips_wiz_bulb_color` | `target?`, `r: 0..255`, `g: 0..255`, `b: 0..255` |
| `philips_wiz_bulb_scene` | `target?`, `scene: string\|int`, `speed?: 10..200` |
| `philips_wiz_bulb_discover` | — |

Registered with Claude Code at **user scope** as `philips-wiz-bulb`
(`claude mcp add philips-wiz-bulb ~/.local/bin/philips-wiz-bulb-mcp -s user`).

Per `feedback_telegram_bot_token_takeover` and lgtv-mcp's lesson:
**a running Claude session must be restarted to pick up new tool schemas** —
MCP reads them at initialize-time. Documented in CLAUDE.md.

## 4. orchctl-v2 integration (mirror `/tv`)

### New files

```
~/orchctl-v2/internal/wiz/
  client.go            # HTTP → philips-wiz-bulb-core
  handler.go           # /bulb slash-command parser
  handler_test.go
~/orchctl-v2/internal/handlers/bulb.go     # makeBulb(deps) adapter
```

### Modified files

```
~/orchctl-v2/internal/handlers/handlers.go   # + /bulb in Commands() and Register()
```

### Slash grammar

```
/bulb                                  → list bulbs + per-bulb state
/bulb on|off                  [target]
/bulb bri    <0-100>          [target]
/bulb temp   <2200-6500>      [target]
/bulb color  <r> <g> <b>      [target]
/bulb scene  <name|id>        [target]
/bulb list | discover
/bulb name   <mac|ip>         <new-name>
```

Replies use HTML formatting (via `telegram.Client.SendMessageHTML`, already
present from the LG TV cutover) — `<b>` for bulb names, `<code>` for MAC/IP.

### Env

`PHILIPS_WIZ_BULB_CORE_URL` (default `http://127.0.0.1:8766`) — orchctl-v2
process only; the daemon does not read this.

## 5. Error handling

| Condition | HTTP | CLI / slash message |
|---|---|---|
| UDP timeout / `EHOSTUNREACH` after one auto-rediscover | 504 | "bulb \<name\> unreachable; retried discovery, still gone" |
| Unknown `target` | 404 | "no bulb matches \<target\>; try `/bulb list`" |
| Out-of-range value | 400 | "\<field\> must be in \<range\>" |
| WiZ error envelope from bulb | 502 | "bulb returned error: \<code\>/\<message\>" |
| Daemon not running | client error | "daemon not running on :8766; `systemctl --user start philips-wiz-bulb-core`" |

## 6. Shared repo conventions (identical to lgtv)

- MIT LICENSE, README.md, CLAUDE.md
- `.github/{workflows/{ci,release}.yml,SECRETS.md,dependabot.yml}`
- gitleaks in CI for secret scanning
- Telegram-notify-on-release in `release.yml` (gated on `TG_BOT_TOKEN` /
  `TG_CHAT_ID` GitHub secrets — both currently unset, so notify is no-op)
- All bulb identifiers come from env or `state.json`; docs use placeholders
  `192.168.1.100` and `aa:bb:cc:dd:ee:ff`
- Go: `unset GOROOT; export GOPROXY=https://proxy.golang.org,direct`
  before any `go` invocation (lgtv build journey lesson — `~/.profile`
  exports an outdated `GOROOT`)
- Git: per-repo `git config user.email
  52166434+deepanshutr@users.noreply.github.com`; never touch global

## 7. Delta vs lgtv pattern (worth noting)

- **No `tg-bot` subcommand in `-cli`**: Telegram lives in orchctl-v2,
  consistent with the post-cutover LG TV state.
- **Daemon port 8766** (lgtv is 8765).
- **Multi-bulb registry**: structural change vs single-device TV; drives the
  `target` parameter through every API/CLI/MCP/slash surface.
- **No pairing step**: WiZ has no equivalent to webOS's pairing — bulbs
  accept LAN `setPilot` calls unauthenticated. Simpler bootstrap, no
  client-key persistence needed.
- **UDP, not TLS WebSocket**: very different transport from webOS; reflected
  in `bulb.py` instead of an `aiowebostv`-style state-bearing client.

## 8. Acceptance

A working delivery means:

1. Daemon discovers `192.168.1.3` on boot, persists `state.json`.
2. `philips-wiz-bulb on`, `... bri 50`, `... temp 3000`, `... color 255 0 0`,
   `... scene cozy`, `... off` all succeed end-to-end against the live bulb.
3. `philips_wiz_bulb_*` MCP tools registered with Claude Code; `claude mcp
   list` shows the server; tools work in a fresh Claude session.
4. `/bulb on`, `/bulb off`, `/bulb scene cozy` work from Telegram via
   orchctl-v2.
5. Bulb IP rolls (simulated by manually clearing `last_ip` in state.json);
   next `setPilot` triggers auto-rediscover and succeeds.
6. All three repos build green on GitHub Actions; gitleaks clean.
7. systemd user unit enables + boots clean across a reboot.
