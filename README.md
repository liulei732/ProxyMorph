# ProxyMorph

ProxyMorph is a self-hosted proxy subscription conversion and orchestration service. The first release focuses on converting Clash subscriptions into Surge 6 subscriptions, while letting you merge fixed pinned nodes into the generated output.

## Current Scope

- Clash subscription URL input.
- Surge 6 subscription output.
- Pinned node library with URI import for `ss://`, `trojan://`, and `vless://`.
- VLESS preservation and sing-box conversion boundary.
- Admin username/password login.
- Embedded management UI.
- SQLite storage under `/data`.
- Docker deployment.

## Docker Compose Quick Start

```bash
docker compose up -d --build
```

Open `http://localhost:28888`, then log in with the configured administrator credentials.

The default compose file uses `network_mode: host` for Linux servers. This keeps `docker ps` readable instead of expanding the full VLESS relay port range. If you run ProxyMorph on Docker Desktop, replace host networking with explicit port mappings for `28888` and `31800-31999`.

Change `PROXYMORPH_ADMIN_PASSWORD` and `PROXYMORPH_SESSION_SECRET` before exposing the service beyond localhost.

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `PROXYMORPH_ADDR` | `:28888` | HTTP listen address. |
| `PROXYMORPH_DATA_DIR` | `/data` | Directory for SQLite database and runtime data. |
| `PROXYMORPH_PUBLIC_BASE_URL` | empty | Base URL used when showing generated subscription links. |
| `PROXYMORPH_ADMIN_USERNAME` | `admin` | Initial administrator username. |
| `PROXYMORPH_ADMIN_PASSWORD` | empty | Initial administrator password. Uses `admin` only when empty during first boot. |
| `PROXYMORPH_SESSION_SECRET` | random dev secret | Secret used to sign admin sessions. Set a stable strong value in production. |
| `PROXYMORPH_VLESS_RELAY_PORT_START` | `31800` | First sing-box relay port used for VLESS helper conversion. |
| `PROXYMORPH_VLESS_RELAY_PORT_END` | `31999` | Last sing-box relay port used for VLESS helper conversion. |

## VLESS And sing-box

The Docker image installs `sing-box`. ProxyMorph preserves VLESS nodes from Clash and fixed-node imports in its normalized node model. Simple VLESS shapes can be rendered directly; richer conversion behavior is isolated behind the sing-box adapter so it can be expanded without changing task or node storage.

For local development, install `sing-box` with your platform package manager if you want to test external conversion behavior.

## Development

```bash
go test ./...
go build ./cmd/proxymorph
```

Frontend:

```bash
cd web
npm install
npm run dev
```

Build frontend assets before building the production binary:

```bash
cd web
npm run build
cd ..
cp -R web/dist/. internal/web/dist/
go build ./cmd/proxymorph
```

## Security Notes

Run ProxyMorph behind HTTPS when exposing it outside a private network. Subscription URLs use bearer-style tokens and should be treated as secrets. Public subscription endpoints avoid detailed internal errors; full diagnostics are shown only in the authenticated admin UI.
