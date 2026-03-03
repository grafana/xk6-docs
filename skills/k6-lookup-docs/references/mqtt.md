# k6/x/mqtt

| I need to... | Path |
|---|---|
| MQTT client API overview | `javascript-api k6-x-mqtt` |
| Client methods | `javascript-api k6-x-mqtt client` |
| Client.connect() | `javascript-api k6-x-mqtt client connect` |
| Register event handlers | `javascript-api k6-x-mqtt client on` |
| Publish messages | `javascript-api k6-x-mqtt client publish` |
| Subscribe to topics | `javascript-api k6-x-mqtt client subscribe` |

Key gotchas:
- Event-driven: `connect`, `message`, `end`, `reconnect`, `error`.
- Supports QoS 0/1/2, retained messages, Last Will.
- Supports `mqtt://`, `mqtts://`, `tcp://`, `tls://`, `ssl://`, `ws://`, `wss://`.
- Async method paths use hyphens: `publish-async` not `publishAsync`.
- Extension module — auto-resolved on import.
