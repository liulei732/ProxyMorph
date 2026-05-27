# Surge Rules, Policy Groups, and Managed Config Design

## Goal

Add configurable Surge policy groups, rule merging, and MANAGED-CONFIG support to ProxyMorph.

## Scope

This feature lets users define reusable global rule settings and per-task rule settings, configure custom policy groups, and generate Surge 6 profiles with optional MANAGED-CONFIG headers.

## Rule Configuration

ProxyMorph supports two rule configuration levels:

- Global rule configuration: the default used by tasks unless overridden.
- Task rule configuration: a task can inherit global rules or use its own rules.

Each rule configuration contains:

- `custom_rules_text`: newline-separated Surge rule lines without a `[Rule]` section header.
- `rule_merge_mode`: how to merge custom rules with upstream subscription rules.

Supported merge modes:

- `custom_first`: custom rules first, then upstream rules, no dedupe.
- `upstream_first`: upstream rules first, then custom rules, no dedupe.
- `custom_first_dedupe`: custom rules first, then upstream rules that did not already appear.
- `upstream_first_dedupe`: upstream rules first, then custom rules that did not already appear.

The renderer always removes blank rules and ensures the final rendered rules end with exactly one `FINAL,<default policy>` line.

## Policy Group Configuration

Each task can define custom Surge policy group lines without a `[Proxy Group]` section header.

First version behavior:

- Preserve upstream policy groups when present.
- Append custom policy groups after upstream policy groups.
- If no upstream policy groups exist, keep the existing automatic `Proxy = select, ...` group.
- Filter upstream group members to available rendered nodes as ProxyMorph already does.

Policy group order controls the default `FINAL` policy because the first rendered group remains the default policy.

## MANAGED-CONFIG

Each task can enable a managed Surge profile header.

Task fields:

- `managed_config_enabled`
- `managed_config_interval_seconds`
- `managed_config_strict`

When enabled, the generated profile starts with:

```ini
#!MANAGED-CONFIG <task subscription URL> interval=<seconds> strict=<true|false>
```

The subscription URL must be URL encoded and must preserve the existing task `name` query parameter.

The MANAGED-CONFIG header must be the first line of the generated file.

## Rendering Order

Generated Surge profile order:

```ini
#!MANAGED-CONFIG ... ; optional

[Proxy]
converted remote nodes
pinned nodes
VLESS relay socks5 nodes

[Proxy Group]
upstream groups or automatic Proxy group
custom task policy groups

[Rule]
merged rules according to rule_merge_mode
FINAL,<default policy>
```

## API And UI

Backend should expose endpoints to read and update global rule configuration and task-level rule/group/managed-config fields.

Frontend should add a configuration surface for:

- Global rules.
- Per-task rule inheritance or override.
- Rule merge mode selection.
- Custom policy group text.
- MANAGED-CONFIG enablement, interval, and strict mode.

The first UI version can use text areas and segmented controls. A visual rule builder can be added later.

## Validation

Backend validation should reject:

- Custom rule text containing `[Rule]`.
- Custom policy group text containing `[Proxy Group]`.
- Empty MANAGED-CONFIG interval or intervals less than 60 seconds.

Backend should normalize:

- Blank lines.
- Repeated `FINAL` lines.
- Leading and trailing whitespace.

## Tests

Required tests:

- Global rule config is used when a task inherits it.
- Task rule config overrides global config.
- All four rule merge modes render in the expected order.
- Dedupe modes preserve the selected priority source.
- MANAGED-CONFIG renders as the first line with encoded task URL.
- Custom policy groups render after upstream groups.
- Renderer still appends `FINAL` when custom and upstream rules omit it.
