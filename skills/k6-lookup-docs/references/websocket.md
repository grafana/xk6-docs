# k6/ws and k6/websockets

Two modules exist: `k6/ws` (callback-based) and `k6/websockets` (class-based).

| I need to... | Path |
|---|---|
| Callback-based API (`k6/ws`) | `javascript-api k6-ws` |
| Class-based API (`k6/websockets`) | `javascript-api k6-websockets` |
| Protocol guide | `using-k6 protocols websockets` |
| Full working example | `examples websockets` |

Key gotchas:
- `k6/ws`: `ws.connect()` blocks until socket closes. ALL logic inside the callback.
- Use `socket.setTimeout()`/`socket.setInterval()` — not `sleep()`.
- Events: `open`, `message`, `binaryMessage`, `close`, `error`, `ping`, `pong`.
