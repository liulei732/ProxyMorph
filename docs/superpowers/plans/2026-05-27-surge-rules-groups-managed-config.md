# Surge Rules, Groups, and Managed Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement global and per-task Surge rule configuration, custom policy groups, and optional MANAGED-CONFIG output.

**Architecture:** Add structured config fields to SQLite and task models, then keep all Surge rendering behavior in `internal/convert`. The subscription service resolves the effective task config, merges rules/groups before rendering, and the React UI exposes text-based controls for global reusable rules and per-task merge behavior.

**Refinement:** Global rule configuration only stores global reusable rule text. Rule merge mode is task-specific only. Existing storage can keep the legacy global merge setting for compatibility, but API and UI should not expose it.

**Tech Stack:** Go, SQLite, React, Vite, embedded frontend assets, Surge 6 profile syntax.

---

### Task 1: Surge Render Options

**Files:**
- Modify: `internal/convert/model.go`
- Modify: `internal/convert/surge.go`
- Test: `internal/convert/converter_test.go`

- [x] Add `SurgeConfig` with `CustomRules`, `CustomGroups`, `RuleMergeMode`, `ManagedConfigHeader`.
- [x] Add rule merge tests for `custom_first`, `upstream_first`, `custom_first_dedupe`, `upstream_first_dedupe`.
- [x] Add tests that custom groups render after upstream groups.
- [x] Add tests that MANAGED-CONFIG renders as the first line.
- [x] Implement rule normalization, dedupe, `FINAL` handling, custom group rendering, and managed header rendering.
- [x] Run `go test ./internal/convert`.

### Task 2: Storage And Models

**Files:**
- Modify: `internal/storage/migrate.go`
- Modify: `internal/storage/models.go`
- Modify: `internal/tasks/service.go`
- Test: `internal/tasks/service_test.go`

- [x] Add `app_settings` table for global rule config.
- [x] Add task columns: `include_global_rules`, `custom_rules_text`, `rule_merge_mode`, `custom_groups_text`, `managed_config_enabled`, `managed_config_interval_seconds`, `managed_config_strict`.
- [x] Extend `storage.ConversionTask` with the new fields.
- [x] Extend task create/update/list/get scan paths.
- [x] Add global rule config service methods in `tasks.Service`.
- [ ] Remove global rule merge mode from API/UI while keeping storage compatibility.
- [x] Test defaults for existing tasks and settings.
- [x] Run `go test ./internal/tasks ./internal/storage`.

### Task 3: Subscription Integration

**Files:**
- Modify: `internal/subscription/service.go`
- Test: `internal/subscription/service_test.go`

- [x] Resolve effective custom rules as global rules plus task rules when `include_global_rules` is true, otherwise task rules only.
- [x] Pass effective custom rules, task custom groups, rule merge mode, and managed header to the Surge renderer.
- [x] Generate MANAGED-CONFIG header from the task subscription URL.
- [x] Test global-included, global-excluded, task custom-only, all four merge modes, custom groups, and managed header.
- [x] Run `go test ./internal/subscription`.

### Task 4: HTTP API

**Files:**
- Modify: `internal/tasks/handlers.go`
- Modify: `internal/app/routes.go`
- Test: `internal/tasks/service_test.go` or new handler tests if needed.

- [x] Add authenticated `GET /api/rule-config` and `PUT /api/rule-config`.
- [x] Extend task `PATCH /api/tasks/{id}` to accept the new per-task fields.
- [x] Return new config fields in task list responses.
- [x] Validate custom rule/group text rejects section headers.
- [x] Run `go test ./internal/tasks ./internal/app`.

### Task 5: Frontend Configuration UI

**Files:**
- Modify: `web/src/main.tsx`
- Modify: `web/src/i18n.ts`
- Modify: `web/src/styles.css`
- Update generated assets under `internal/web/dist/`
- Test: `web/src/i18n.test.ts`

- [x] Add `RuleConfig` and extended `Task` TypeScript types.
- [x] Fetch global rule config during refresh.
- [x] Add Settings panel controls for global rules.
- [x] Add per-task advanced config controls: include global rules, merge mode, custom rules, custom groups, MANAGED-CONFIG interval/strict.
- [x] Add save toasts and error toasts for global and task config.
- [x] Build frontend and copy `web/dist` to `internal/web/dist`.
- [x] Run `node --experimental-strip-types --test web/src/i18n.test.ts` and `npm run build`.

### Task 6: Final Verification

**Files:**
- All changed files.

- [x] Run `go test ./...`.
- [x] Run `node --experimental-strip-types --test web/src/i18n.test.ts`.
- [x] Run `npm run build`.
- [x] Run `docker build -t proxymorph:dev .`.
- [x] Inspect `git diff --stat`.
- [ ] Commit with `feat: add surge rule configuration`.
- [ ] Push `main`.
