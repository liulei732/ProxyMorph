# Detailed MANAGED-CONFIG Design

## Goal

Upgrade ProxyMorph MANAGED-CONFIG from a simple per-task toggle into a structured configuration surface with global defaults, task-level overrides, URL handling, friendly refresh presets, and final header preview.

## Background

Surge managed profiles use a first-line header:

```ini
#!MANAGED-CONFIG <url> interval=<seconds> strict=<true|false>
```

ProxyMorph should keep the generated header compatible with Surge and only emit officially supported header parameters. More detailed behavior should live in ProxyMorph's own settings and resolve into the standard header.

## Configuration Model

Add a global MANAGED-CONFIG default in `app_settings`:

- `managed_config_enabled`: default enabled state.
- `managed_config_url_mode`: `task_subscription` or `custom`.
- `managed_config_custom_url`: custom public URL used when URL mode is `custom`.
- `managed_config_interval_seconds`: refresh interval in seconds.
- `managed_config_strict`: strict mode default.

Each conversion task stores a mode and optional overrides:

- `managed_config_mode`: `global`, `enabled`, or `disabled`.
- `managed_config_url_mode`: `global`, `task_subscription`, or `custom`.
- `managed_config_custom_url`: task custom public URL.
- `managed_config_interval_mode`: `global` or `custom`.
- `managed_config_interval_seconds`: task custom refresh interval.
- `managed_config_strict_mode`: `global`, `enabled`, or `disabled`.

Effective configuration resolution:

- Enabled state:
  - `enabled`: enabled for the task.
  - `disabled`: disabled for the task.
  - `global`: follows the global default.
- URL:
  - `task_subscription`: use the task subscription URL.
  - `custom`: use the configured custom URL.
  - `global`: use the global URL mode and value.
- Interval:
  - `custom`: use task interval.
  - `global`: use global interval.
- Strict:
  - `enabled`: render `strict=true`.
  - `disabled`: render `strict=false`.
  - `global`: follows global strict default.

Existing task fields `managed_config_enabled`, `managed_config_interval_seconds`, and `managed_config_strict` can be replaced because the project does not require old-version compatibility.

## URL Rules

When URL mode resolves to `task_subscription`, ProxyMorph uses the existing task subscription URL builder. It must preserve the task `name` query parameter and URL encode it correctly.

When URL mode resolves to `custom`, ProxyMorph validates:

- URL must start with `http://` or `https://`.
- URL must be absolute.
- URL must not be empty when the effective managed config is enabled.

For custom URLs, ProxyMorph must not silently append `name`. The configured custom URL is rendered exactly as entered after validation.

## Refresh Interval UI

The frontend should show interval presets:

- `1 hour`: 3600
- `6 hours`: 21600
- `12 hours`: 43200
- `24 hours`: 86400
- `Custom`: user-entered seconds

Validation rules:

- Minimum interval: 60 seconds.
- Empty custom interval is rejected when the effective interval mode is custom.
- Global default interval uses 86400 seconds when unset.

## Strict Mode UI

Strict mode should remain a boolean effective value, but the frontend should explain it in plain language:

- Chinese: `严格模式会让 Surge 更严格地校验托管配置更新，配置源稳定时再启用。`
- English: `Strict mode asks Surge to validate managed profile updates more strictly. Enable it only when the profile source is stable.`

Default should be `false`.

## Header Preview

Task editing should include a read-only preview of the effective header when MANAGED-CONFIG is enabled:

```ini
#!MANAGED-CONFIG https://example.com/sub/token?name=Main interval=86400 strict=false
```

If disabled, show `MANAGED-CONFIG disabled`.

The preview can be computed by the frontend from task and global config values, but the backend remains the source of truth during generation.

## API

Add authenticated endpoints:

- `GET /api/managed-config-defaults`
- `PUT /api/managed-config-defaults`

Extend task create/update/list responses with the task-level managed config fields.

The backend should normalize unknown enum values to safe defaults:

- Task mode defaults to `global`.
- URL mode defaults to `global` for tasks and `task_subscription` globally.
- Interval mode defaults to `global`.
- Strict mode defaults to `global`.

## Rendering Behavior

When the effective managed config is enabled, generated Surge output starts with:

```ini
#!MANAGED-CONFIG <effective-url> interval=<effective-interval-seconds> strict=<effective-strict>
```

This line must remain the first line of the generated profile.

When disabled, no MANAGED-CONFIG header is rendered.

## Tests

Required tests:

- Global defaults round trip through storage and API.
- Task `global` mode follows the global enabled setting.
- Task `enabled` overrides disabled global default.
- Task `disabled` overrides enabled global default.
- Task custom URL renders exactly as configured.
- Task subscription URL preserves encoded `name`.
- Custom interval overrides global interval.
- Strict mode task override renders `strict=true` or `strict=false`.
- Invalid custom URL is rejected.
- Intervals below 60 seconds are rejected.
- Header remains the first generated line.

## Out Of Scope

- Emitting non-Surge MANAGED-CONFIG header parameters.
- Automatically appending query parameters to custom URLs.
- Per-device managed config profiles.
- Visual rule builder changes.
