---
name: xk6-docs
description: Look up k6 documentation to write k6 load tests, browser tests, and performance scripts. Use when writing k6 test scripts, looking up k6 APIs (http, browser, websockets, grpc, metrics), or understanding k6 concepts (thresholds, checks, scenarios, executors).
---

# k6 Documentation Lookup

Read k6 docs via `<binary> x docs` to write correct k6 test scripts. Docs are local and cached.

**If `<binary>` is not found:** tell the user to re-run `k6 x docs skill <dir>` to update the skill, then stop. Do not attempt fallbacks.

## Command Syntax

All lookups use `<binary> x docs` followed by a topic path. Paths in this skill and its references omit the `<binary> x docs` prefix.

```
<binary> x docs <parent> <child>         # Read a topic (requires full parent path)
<binary> x docs <parent> <child> <leaf>  # Read a nested topic
<binary> x docs --depth 1                # TOC: top-level categories only (default)
<binary> x docs --depth 2                # TOC: one level of children
<binary> x docs search <term>            # Fuzzy search (returns topic paths, not content)
```

## Critical Rules

- **Topics require their full parent path.** `using-k6 thresholds` works. `thresholds` alone FAILS.
- **Every topic page lists its subtopics at the bottom.** Use these to navigate deeper.
- **Use `--depth 1`** to explore a category without reading content.
- **Avoid `search`** unless direct paths fail — it returns many results.

## Workflows

Read the reference that matches your task. Each has lookup paths and key gotchas.

| Task | Reference |
|---|---|
| HTTP testing | [references/http.md](references/http.md) |
| Browser testing (+ hybrid browser+protocol) | [references/browser.md](references/browser.md) |
| gRPC testing | [references/grpc.md](references/grpc.md) |
| WebSocket testing | [references/websocket.md](references/websocket.md) |
| MQTT testing | [references/mqtt.md](references/mqtt.md) |
| Redis | [references/redis.md](references/redis.md) |
| DNS resolution | [references/dns.md](references/dns.md) |
| ICMP ping | [references/icmp.md](references/icmp.md) |
| TLS / SSL | [references/tls.md](references/tls.md) |
| Crypto (hashing, HMAC, Web Crypto) | [references/crypto.md](references/crypto.md) |
| SharedArray (k6/data) | [references/data.md](references/data.md) |
| Execution context (VU, scenario, iteration) | [references/execution.md](references/execution.md) |
| Custom metrics (Counter, Gauge, Rate, Trend) | [references/metrics.md](references/metrics.md) |
| Base64 encoding | [references/encoding.md](references/encoding.md) |
| HTML parsing | [references/html.md](references/html.md) |
| Timers (setTimeout, setInterval) | [references/timers.md](references/timers.md) |
| Secrets | [references/secrets.md](references/secrets.md) |
| Experimental (CSV, filesystem, streams) | [references/experimental.md](references/experimental.md) |
| jslib (httpx, k6chaijs, utils, aws, etc.) | [references/jslib.md](references/jslib.md) |
| Error codes | [references/error-codes.md](references/error-codes.md) |
| Scenarios, thresholds, checks, options, lifecycle | [references/test-config.md](references/test-config.md) |
| Running tests, output backends, dashboards | [references/running-and-output.md](references/running-and-output.md) |
| Test authoring, modules, extensions | [references/authoring-and-modules.md](references/authoring-and-modules.md) |
| Examples and test type guides | [references/examples-and-guides.md](references/examples-and-guides.md) |

## Module API Quick Reference

All module APIs live under `javascript-api`. Names use hyphens, not slashes. Drill into a function: `javascript-api <module> <function>`.

### Core modules

| k6 import | Lookup path | Contains |
|---|---|---|
| `k6` | `javascript-api k6` | `check`, `sleep`, `group`, `fail` |
| `k6/http` | `javascript-api k6-http` | `get`, `post`, `put`, `del`, `batch`, `request`; `Response`, `Params`, `CookieJar` |
| `k6/browser` | `javascript-api k6-browser` | `browser.newPage()`, `Page`, `Locator`, `BrowserContext` |
| `k6/ws` | `javascript-api k6-ws` | Callback-based WebSocket: `connect()`, `Socket` |
| `k6/websockets` | `javascript-api k6-websockets` | Class-based WebSocket API |
| `k6/net/grpc` | `javascript-api k6-net-grpc` | `Client`, `Stream`, status constants |
| `k6/data` | `javascript-api k6-data` | `SharedArray` — memory-efficient data sharing across VUs |
| `k6/execution` | `javascript-api k6-execution` | Runtime info: current VU, scenario, iteration, test state |
| `k6/metrics` | `javascript-api k6-metrics` | Custom metrics: `Counter`, `Gauge`, `Rate`, `Trend` |
| `k6/crypto` | `javascript-api k6-crypto` | Hashing and HMAC |
| `k6/encoding` | `javascript-api k6-encoding` | Base64 encode/decode |
| `k6/html` | `javascript-api k6-html` | HTML parsing and CSS selection |
| `k6/timers` | `javascript-api k6-timers` | `setTimeout`, `setInterval` (also available globally) |
| `k6/secrets` | `javascript-api k6-secrets` | Access secrets from configured secret sources |

### Extension modules (`k6/x/*`)

| k6 import | Lookup path | Contains |
|---|---|---|
| `k6/x/mqtt` | `javascript-api k6-x-mqtt` | MQTT client: connect, publish, subscribe (event-driven, QoS 0/1/2) |
| `k6/x/redis` | `javascript-api k6-x-redis` | Redis client: get/set/del, lists, hashes, sets, pub/sub |
| `k6/x/dns` | `javascript-api k6-x-dns` | DNS resolution: `resolve()`, `lookup()`, A/AAAA records |
| `k6/x/icmp` | `javascript-api k6-x-icmp` | ICMP ping: latency and reachability checks |
| `k6/x/tls` | `javascript-api k6-x-tls` | TLS certificate inspection and validation |

### Other APIs

| API | Lookup path | Contains |
|---|---|---|
| `k6/experimental` | `javascript-api k6-experimental` | Experimental: CSV parsing, filesystem, streams |
| Web Crypto | `javascript-api crypto` | `getRandomValues`, `randomUUID`, `subtle` (encrypt/decrypt/sign) |
| jslib | `javascript-api jslib` | URL-imported libs: `httpx`, `k6chaijs`, `utils`, `aws`, `testing` |
| Init context | `javascript-api init-context` | `open()` — read local files during init stage |
| Error codes | `javascript-api error-codes` | Numeric codes for non-HTTP errors (DNS, TLS, timeout, etc.) |
