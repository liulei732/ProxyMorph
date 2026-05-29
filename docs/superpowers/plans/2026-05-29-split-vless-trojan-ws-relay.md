# Split VLESS and Trojan WS Relay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Let VLESS and Trojan WebSocket sing-box relay conversion be configured independently at global and task level.

**Architecture:** Keep one sing-box manager/process and one relay port pool, but select relay candidates by independent effective modes. VLESS nodes and Trojan WebSocket nodes are persisted in the existing relay table with node JSON, converted to SOCKS5 nodes for Surge, and restored after restart. Trojan non-WS nodes remain static Surge proxy lines.

**Tech Stack:** Go backend, SQLite migrations, sing-box JSON generation, React/TypeScript frontend, Go and npm tests.

---

### Task 1: Backend Configuration and Storage Shape

**Files:**
- Modify: `internal/storage/migrate.go`
- Modify: `internal/storage/models.go`
- Modify: `internal/tasks/service.go`
- Modify: `internal/tasks/service_test.go`
- Modify: `internal/app/app_test.go`

- [x] Add `trojan_ws_relay_mode` to `conversion_tasks` with default `global`.
- [x] Add `TrojanWSRelayMode` to `storage.ConversionTask`.
- [x] Add `TrojanWSRelayEnabled` to `storage.GlobalRuleConfig` and tasks API input.
- [x] Read and write `trojan_ws_relay_enabled` in global settings.
- [x] Normalize task modes through the existing tri-state helper.
- [x] Update tests for task create/update and global settings round trip.

### Task 2: Preserve UDP Metadata

**Files:**
- Modify: `internal/nodes/parser.go`
- Modify: `internal/convert/clash.go`
- Modify: `internal/nodes/parser_test.go`
- Modify: `internal/convert/converter_test.go`

- [x] Parse Trojan URI `udp` as enabled by default unless explicitly false.
- [x] Parse Clash `udp: true` for Trojan, VLESS, VMess, and AnyTLS.
- [x] Render `udp-relay=true` through the existing shared Surge renderer.
- [x] Keep existing AnyTLS behavior unchanged.

### Task 3: Relay Selection and sing-box Outbounds

**Files:**
- Modify: `internal/singbox/manager.go`
- Modify: `internal/singbox/adapter_test.go`
- Modify: `internal/subscription/service.go`
- Modify: `internal/subscription/service_test.go`

- [x] Rename internal relay helpers from VLESS-specific behavior to generic protocol relay behavior while preserving public method compatibility where practical.
- [x] Treat VLESS as relayable when effective VLESS mode is enabled.
- [x] Treat Trojan WebSocket as relayable when effective Trojan WS mode is enabled.
- [x] Generate sing-box Trojan outbound with password, TLS server name, insecure flag, UDP network, and WebSocket transport headers/path.
- [x] Persist and restore both relay node types.
- [x] Replace relay nodes with SOCKS5 Surge nodes containing `relay_protocol`.
- [x] Clear relay entries when neither effective relay mode is enabled.

### Task 4: Frontend Controls

**Files:**
- Modify: `web/src/managedPreview.ts`
- Modify: `web/src/main.tsx`
- Modify: `web/src/i18n.ts`
- Modify: relevant `web/src/*.test.ts`

- [x] Extend task and rule config types with Trojan WS relay fields.
- [x] Add global settings checkbox for Trojan WebSocket auxiliary conversion.
- [x] Add task editor conversion row for Trojan WebSocket auxiliary conversion.
- [x] Show both relay modes in the conversion summary.
- [x] Keep preview refresh behavior on every change.
- [x] Add/update focused frontend tests for draft binding and summary output.

### Task 5: Verification

**Files:**
- Modify as needed from prior tasks.

- [x] Run `gofmt` on changed Go files.
- [x] Run `go test ./...`.
- [x] Run `npm test` in `web`.
- [x] Run `npm run build` in `web` and rebuild embedded web dist if the project build expects it.
- [x] Inspect `git diff` to ensure `.DS_Store` is not included.

