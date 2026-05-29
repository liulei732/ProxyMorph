# ProxyMorph Troubleshooting Notes

This document records issues that have already been diagnosed in ProxyMorph so we do not repeat the same investigation later.

## Trojan WebSocket Direct Surge Output

### Known Working Shape

When Trojan WebSocket helper conversion is not enabled, a working direct Surge node can look like this:

```text
越南-CMI专线1 = trojan, resolution1.private.berry-is-sweet.com, 2053, password=..., sni=17800372086633.berrycdn-3.com, udp-relay=true, ws=true, ws-path=/videotahyjhghmuaawe, ws-headers=Host:j1.berrycdn-3.com
```

Important fields:

- `sni` must prefer the original node `sni` value when present.
- `peer` must not overwrite `sni`; if both exist and differ, keep `sni` for Surge output.
- `ws-headers=Host:...` must preserve the original WebSocket Host header.
- `udp-relay=true` is valid Surge metadata and should be preserved for Trojan nodes when UDP is enabled or unspecified in Trojan URI input.

### Earlier Failure Modes

- Using `peer` as `sni` caused TLS certificate mismatch errors in Surge.
- Using a generated or changed SNI could produce certificate errors or WebSocket failures.
- Removing or changing the WebSocket Host header could cause `websocket close`.
- Seeing `trojan, ... ws=true ...` in output is expected when Trojan WebSocket helper conversion is disabled.

## Trojan WebSocket Helper Conversion

Trojan WebSocket helper conversion is optional. It is mainly a fallback for nodes that Surge cannot connect to directly.

When helper conversion is enabled and effective for a task, the generated Surge node should no longer be a direct `trojan` line. It should become a `socks5` relay node, for example:

```text
越南-CMI专线1 = socks5, proxy.example.com, 31800, username=relay, password=...
```

Checklist when the output is still a direct `trojan` line:

- Confirm global setting `trojan_ws_relay_enabled` is enabled.
- Confirm the task `trojan_ws_relay_mode` is `enabled`, or is `global` while the global setting is enabled.
- Confirm the deployment is running the latest image/code.
- Confirm sing-box relay environment is enabled and has a valid public host and port range.

## VLESS And Trojan WebSocket Relay Separation

VLESS and Trojan WebSocket helper conversion are separate settings.

- VLESS relay only affects `vless` nodes.
- Trojan WebSocket relay only affects `trojan` nodes where `network=ws`.
- Plain Trojan nodes must remain direct Surge output.
- Both relay types share one sing-box manager and one relay port pool.

Task-level modes:

- `global`: follow global setting.
- `enabled`: force helper conversion on for this task.
- `disabled`: force helper conversion off for this task.

## sing-box Trojan WebSocket Outbound

For Trojan WebSocket relay, the sing-box outbound must keep WebSocket transport over TCP.

Do not set the sing-box Trojan outbound `network` to `udp` just because the original node has UDP enabled. WebSocket itself is TCP-based, so the outbound should use `network: "tcp"` with a WebSocket transport block.

The original UDP capability should still be preserved in the normalized node model and direct Surge output as `udp-relay=true`.

## Deployment Checks

If local code is correct but server output is old:

- Pull the latest commit or rebuild the Docker image.
- Confirm embedded frontend assets were rebuilt from `web/dist` into `internal/web/dist`.
- Confirm the running container was recreated, not only restarted.
- If relay ports changed, make sure old sing-box processes are stopped and no stale relay port is occupied.
