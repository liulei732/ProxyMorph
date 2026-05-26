# ProxyMorph Design

Date: 2026-05-26

## Product Goal

ProxyMorph is a lightweight proxy subscription conversion and orchestration service. The first release converts Clash subscriptions into Surge 6 subscriptions, while keeping the domain model open for future input and output formats.

The service is designed for personal or small self-hosted deployment first. Version 1 supports one administrator account, but data ownership is modeled with `user_id` fields so multi-user support can be added later without rewriting the storage layer.

## Scope

Version 1 includes:

- A Go backend delivered as a single binary.
- Embedded frontend assets served by the Go process.
- Docker deployment with persistent data under `/data`.
- SQLite storage.
- Administrator username and password login.
- Management UI for subscription conversion tasks.
- Clash subscription URL input.
- Surge 6 subscription URL output.
- A pinned node library for fixed proxy nodes.
- URI import and form editing for pinned nodes.
- Task-level controls for merging pinned nodes into converted output.
- Conversion preview and error visibility in the admin UI.

Version 1 does not include:

- Full multi-user account management.
- Team or role-based access control.
- Billing, public registration, or invite flows.
- Conversion implementations beyond Clash to Surge 6.
- Advanced rule editing beyond conversion options needed for the first release.

## Architecture

ProxyMorph runs as one Go service. The backend exposes admin APIs, public subscription endpoints, static frontend assets, and health checks from the same process.

Primary backend modules:

- `auth`: Handles administrator login, session or JWT issuance, password hashing, and auth middleware.
- `admin_api`: Provides authenticated JSON APIs used by the management UI.
- `source`: Fetches upstream subscription content and tracks fetch metadata.
- `converter`: Defines conversion interfaces and contains the first Clash to Surge 6 converter.
- `proxy_parser`: Parses proxy URI formats into the internal node model.
- `node_library`: Stores and manages pinned nodes.
- `node_merge`: Combines remote subscription nodes with pinned nodes.
- `subscription`: Serves tokenized public Surge 6 subscription URLs.
- `storage`: Owns SQLite schema, migrations, and persistence APIs.
- `web`: Embeds and serves built frontend assets.
- `config`: Loads environment variables and runtime settings.

The frontend can be implemented with a modern TypeScript UI stack, built into static assets, and embedded into the Go binary. The backend remains usable without a separate frontend server in production.

## Deployment Model

Docker is the primary deployment target.

Expected container behavior:

- Listen on a configurable port, defaulting to `8080`.
- Store SQLite database, caches, and runtime files under `/data`.
- Accept initial administrator credentials from environment variables on first boot.
- Preserve existing credentials after initialization unless explicitly reset.
- Support a configurable public base URL for generated subscription links.

Important environment variables:

- `PROXYMORPH_ADDR`: HTTP listen address, such as `:8080`.
- `PROXYMORPH_DATA_DIR`: Data directory, default `/data`.
- `PROXYMORPH_PUBLIC_BASE_URL`: Base URL used when showing generated subscription links.
- `PROXYMORPH_ADMIN_USERNAME`: Initial administrator username.
- `PROXYMORPH_ADMIN_PASSWORD`: Initial administrator password.
- `PROXYMORPH_SESSION_SECRET`: Secret used to sign sessions or tokens.

## Data Model

Core entities:

- `users`: Administrator account records. Version 1 creates one admin user.
- `conversion_tasks`: Subscription conversion tasks owned by a user.
- `pinned_nodes`: Fixed proxy nodes owned by a user.
- `task_node_overrides`: Per-task pinned node include and exclude rules.
- `subscription_tokens`: Public tokens for generated subscription URLs.
- `conversion_runs`: Fetch, merge, conversion, and error history.
- `output_cache`: Last successful generated Surge 6 output for each task.

`conversion_tasks` fields include:

- `id`
- `user_id`
- `name`
- `input_type`, initially `clash`
- `output_type`, initially `surge6`
- `source_url`
- `enabled`
- `refresh_interval_seconds`
- `merge_default_pinned_nodes`
- `pinned_node_order_mode`
- `last_success_at`
- `last_error_at`
- `last_error_message`

`pinned_nodes` fields include:

- `id`
- `user_id`
- `name`
- `protocol`
- `server`
- `port`
- `parameters_json`
- `tags_json`
- `enabled`
- `default_include`
- `sort_order`
- `created_at`
- `updated_at`

Protocol-specific values such as cipher, password, TLS, SNI, network type, WebSocket path, and plugin options live in `parameters_json`. This keeps the first schema flexible while still giving the UI typed forms for common protocols.

## Pinned Nodes

Pinned nodes are fixed proxy nodes that can be added to converted subscriptions. They are useful for self-hosted nodes, backup lines, special-purpose routes, or nodes that should not depend on the upstream Clash subscription.

The admin UI supports two node entry paths:

- URI import: Paste one or more proxy URIs such as `ss://`, `vmess://`, or `trojan://`. ProxyMorph parses each URI into an internal node record.
- Form editing: Edit node name, protocol, server, port, credentials, TLS/SNI, network options, tags, enabled state, default inclusion, and sort order.

Pinned node association works at two levels:

- A pinned node can be marked `default_include`, which makes it available to all tasks that merge default pinned nodes.
- A task can explicitly include additional pinned nodes or exclude default pinned nodes.

The effective pinned nodes for a task are:

1. Enabled default pinned nodes when `merge_default_pinned_nodes` is enabled.
2. Enabled pinned nodes explicitly included by the task.
3. Minus pinned nodes explicitly excluded by the task.

## Conversion Flow

When a Surge 6 subscription URL is requested:

1. Validate the subscription token and find the conversion task.
2. Return a concise error if the task is disabled or missing.
3. Load the last successful output cache.
4. Refresh upstream Clash content if the cache is missing or expired.
5. Parse Clash YAML into an internal representation of nodes, proxy groups, and rules.
6. Load effective pinned nodes for the task.
7. Normalize remote nodes and pinned nodes into a shared internal node model.
8. Merge nodes according to task settings.
9. Resolve duplicates and naming conflicts.
10. Render Surge 6 configuration text.
11. Store the successful output and conversion run metadata.
12. Return the generated Surge 6 content.

If upstream fetch or conversion fails, ProxyMorph returns the last successful cached output when available and records the error for the admin UI. If no cache exists, the subscription endpoint returns a concise failure response.

## Merge Rules

Node merging is deterministic.

Default behavior:

- Remote subscription nodes are kept in upstream order.
- Pinned nodes are inserted after remote nodes unless the task selects another insertion mode.
- Disabled pinned nodes are ignored.
- Exact duplicate nodes are collapsed.
- Name conflicts are resolved by preserving the first name and suffixing later entries, such as `Node`, `Node 2`, `Node 3`.

Task-level insertion modes:

- `after_remote`: Append pinned nodes after upstream nodes.
- `before_remote`: Place pinned nodes before upstream nodes.
- `sort_order`: Combine remote and pinned nodes using explicit pinned node order plus stable remote order.

The first release should favor predictable behavior over aggressive automatic optimization.

## Admin UI

The admin interface should feel like a quiet, polished operations tool. It should emphasize scanability, direct controls, clear status, and fast copy actions.

Primary screens:

- Login page.
- Dashboard with task count, recent conversion status, and current errors.
- Conversion task list with name, input type, output type, enabled state, last refresh, and copyable subscription URL.
- Task create/edit page with source URL, refresh interval, output type, enabled state, token controls, and pinned node merge settings.
- Pinned node list with protocol, name, tags, default inclusion, enabled state, and sorting.
- Pinned node import dialog for one or more proxy URIs.
- Pinned node edit form for protocol-specific fields.
- Conversion preview showing upstream nodes, pinned nodes, final merged nodes, and rendered Surge 6 output.
- Logs page showing recent fetch, parse, merge, and render events.
- Settings page for public base URL, admin password, and cache behavior.

The UI should not require users to understand internal architecture. It should make common workflows obvious: create a task, copy the Surge subscription URL, add fixed nodes, preview output, and diagnose conversion errors.

## Authentication And Security

Version 1 uses one administrator account.

Security requirements:

- Store passwords with a modern password hashing algorithm.
- Protect admin APIs with authenticated sessions or signed tokens.
- Use secure cookie settings when session cookies are used.
- Subscription URLs use unguessable tokens and do not require login.
- Do not expose detailed internal errors from public subscription endpoints.
- Redact secrets in logs and UI previews where appropriate.
- Avoid writing plaintext administrator passwords after initialization.

The first release can assume the service is deployed behind HTTPS by a reverse proxy. Documentation should make that expectation explicit.

## Error Handling

ProxyMorph records errors at the task and run level.

Error categories:

- Upstream fetch failed.
- Upstream response was empty or invalid.
- Clash YAML parse failed.
- Proxy URI parse failed.
- Unsupported node protocol or unsupported protocol option.
- Surge 6 render failed.
- Cache unavailable.

Admin UI errors should be actionable and tied to the relevant task or node. Public subscription endpoint errors should be brief and avoid leaking sensitive configuration.

## Testing Strategy

Backend tests:

- Clash fixture parsing.
- Surge 6 rendering from normalized internal nodes.
- Pinned node URI parsing.
- Pinned node merge behavior, including default include, explicit include, explicit exclude, disabled nodes, duplicate nodes, and name conflicts.
- Subscription cache behavior on successful and failed refreshes.
- Authentication login and protected API access.

Integration tests:

- Start the service with a temporary SQLite database.
- Create an admin session.
- Create a conversion task.
- Import pinned nodes from URIs.
- Generate a tokenized subscription URL.
- Fetch the Surge 6 output and assert that remote and pinned nodes appear.

Docker smoke test:

- Build the Docker image.
- Run the container with `/data` mounted.
- Confirm health endpoint responds.
- Confirm first boot initializes the admin account.
- Confirm data persists across restart.

## Future Expansion

The architecture should make these future additions natural:

- Additional input formats such as Sing-box, V2Ray, plain proxy URI lists, and manual YAML.
- Additional output formats beyond Surge 6.
- Full multi-user account management.
- Scheduled background refresh instead of refresh-on-request only.
- Rule and policy group customization.
- Import/export backup of tasks and pinned nodes.
- Webhook or notification support for repeated conversion failures.

## Open Decisions Resolved For Version 1

- The project name is ProxyMorph.
- The backend is Go.
- Production packaging is a single Go binary with embedded frontend assets.
- Docker is a first-class deployment path.
- SQLite is the initial database.
- Version 1 supports one administrator account while reserving multi-user data boundaries.
- Version 1 implements Clash to Surge 6 conversion only.
- Fixed nodes are supported through a pinned node library.
- Pinned nodes can be imported from URI and edited through forms.
- Pinned nodes can be default-included globally and overridden per task.
