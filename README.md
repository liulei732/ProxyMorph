# ProxyMorph

English | [中文](README.zh-CN.md)

ProxyMorph is a self-hosted proxy subscription conversion service. The current release focuses on converting Clash subscriptions into Surge 6 profiles, merging pinned nodes, managing custom rules and policy groups, and providing optional sing-box helper relays for protocols that need a compatibility bridge.

## Features

- Convert Clash subscription URLs into Surge 6 subscriptions.
- Manage multiple conversion tasks from the embedded web UI.
- Import and merge pinned nodes from proxy URIs or Surge proxy lines.
- Support `ss`, `trojan`, `vmess`, `vless`, and `anytls` in the normalized node model where applicable.
- Preserve Trojan WebSocket SNI, WebSocket Host, path, and UDP metadata for Surge output.
- Configure custom rules, custom policy groups, rule merge order, and FINAL policy.
- Generate MANAGED-CONFIG headers with live preview.
- Preview task output before saving and keep previews refreshed while editing.
- Optional sing-box helper relay for VLESS and Trojan WebSocket nodes, configured separately.
- Authenticated admin UI with password change support.
- SQLite storage under `/data`.
- Docker deployment with embedded frontend assets and bundled `sing-box`.

## Project Layout

```text
cmd/proxymorph/      Application entry point
internal/app/        HTTP routing and app wiring
internal/auth/       Admin authentication and password handling
internal/config/     Environment-based runtime config
internal/convert/    Clash parsing, normalized model, Surge rendering
internal/nodes/      Pinned node import and management
internal/singbox/    sing-box relay manager and config generation
internal/storage/    SQLite schema, migrations, models
internal/subscription/ Subscription generation and caching
internal/tasks/      Conversion task service and handlers
internal/web/        Embedded frontend assets
web/                 React/Vite management UI
docs/                Security notes, troubleshooting, and planning docs
```

## Docker Compose Quick Start

1. Edit `docker-compose.yml`.
2. Change the administrator password, session secret, relay password, public URL, and relay public host.
3. Start the service:

```bash
docker compose up -d --build
```

Open `http://localhost:28888`, then sign in with the configured administrator account.

The default compose file uses `network_mode: host` for Linux servers. This keeps `docker ps` readable instead of listing hundreds of relay port mappings. If you run ProxyMorph on Docker Desktop, replace host networking with explicit port mappings for `28888` and the relay range `31800-31999`.

## Important Deployment Notes

- Use HTTPS before exposing the admin UI or subscription URLs to the public internet.
- Change `PROXYMORPH_ADMIN_PASSWORD`, `PROXYMORPH_SESSION_SECRET`, and `PROXYMORPH_VLESS_RELAY_PASSWORD` before deployment.
- Set `PROXYMORPH_PUBLIC_BASE_URL` to the real external address, for example `https://proxy.example.com`.
- Set `PROXYMORPH_VLESS_RELAY_PUBLIC_HOST` to the host that Surge devices can reach.
- Keep `/data` persistent. It stores the SQLite database, cached outputs, and sing-box runtime config.
- Recreate the container after pulling a new image; do not rely on restarting an old container.

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `PROXYMORPH_ADDR` | `:28888` | HTTP listen address. |
| `PROXYMORPH_DATA_DIR` | `/data` | Directory for SQLite database and runtime data. |
| `PROXYMORPH_PUBLIC_BASE_URL` | empty | Base URL used for generated subscription links. |
| `PROXYMORPH_ADMIN_USERNAME` | `admin` | Initial administrator username. |
| `PROXYMORPH_ADMIN_PASSWORD` | empty | Initial administrator password. Uses `admin` only when empty during first boot. |
| `PROXYMORPH_SESSION_SECRET` | random dev secret | Secret used to sign admin sessions. Set a stable strong value in production. |
| `PROXYMORPH_VLESS_RELAY_ENABLED` | `false` | Enables the sing-box relay environment. Task/global UI settings still decide which nodes use it. |
| `PROXYMORPH_VLESS_RELAY_PUBLIC_HOST` | derived from public URL | Host used in generated SOCKS5 relay nodes. |
| `PROXYMORPH_VLESS_RELAY_LISTEN_HOST` | `0.0.0.0` | Host that sing-box SOCKS inbounds listen on. |
| `PROXYMORPH_VLESS_RELAY_PORT_START` | `31800` | First sing-box relay port. |
| `PROXYMORPH_VLESS_RELAY_PORT_END` | `31999` | Last sing-box relay port. |
| `PROXYMORPH_VLESS_RELAY_USERNAME` | `proxymorph` | Username for generated SOCKS5 relay nodes. |
| `PROXYMORPH_VLESS_RELAY_PASSWORD` | empty | Password for generated SOCKS5 relay nodes. Set a strong value in production. |
| `PROXYMORPH_SING_BOX_PATH` | `sing-box` | sing-box executable path. |
| `PROXYMORPH_SING_BOX_CONFIG_PATH` | `/data/sing-box.json` | Generated sing-box config path. |

## Helper Relay Behavior

ProxyMorph has two independent helper conversion switches in the UI:

- VLESS helper conversion affects only `vless` nodes.
- Trojan WebSocket helper conversion affects only `trojan` nodes with WebSocket transport.

When a helper relay is effective for a task, the generated Surge node becomes a `socks5` relay node that points to the server. sing-box then connects to the original upstream node. Plain Trojan nodes remain direct Surge output.

Trojan WebSocket direct output is expected to remain a `trojan` line when Trojan WebSocket helper conversion is disabled. See [Troubleshooting](docs/troubleshooting.md) for the SNI, WebSocket Host, `udp-relay`, and helper relay details already diagnosed during testing.

## Development

Backend tests:

```bash
go test ./...
```

Frontend development:

```bash
cd web
npm install
npm run dev
```

Build embedded frontend assets before building a production binary locally:

```bash
cd web
npm run build
cd ..
rm -rf internal/web/dist
mkdir -p internal/web/dist
cp -R web/dist/. internal/web/dist/
go build ./cmd/proxymorph
```

Recommended verification before committing:

```bash
cd web && node --test src/*.test.ts
cd web && npm run build
go test ./...
```

## Documentation

- [Chinese README](README.zh-CN.md)
- [Troubleshooting notes](docs/troubleshooting.md)
- [Security review](docs/security-review.md)
- [Planning policy](docs/planning-policy.md)

## Security Notes

ProxyMorph is intended to be self-hosted by a trusted administrator. Subscription URLs contain bearer-style tokens and should be treated as secrets. Public subscription endpoints intentionally avoid detailed internal errors; detailed diagnostics are shown only in the authenticated admin UI.

Review [docs/security-review.md](docs/security-review.md) before publishing a deployment to the public internet.
