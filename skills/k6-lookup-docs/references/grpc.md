# k6/net/grpc

| I need to... | Path |
|---|---|
| Overview with all streaming mode examples | `using-k6 protocols grpc` |
| Client, Stream, status constants | `javascript-api k6-net-grpc` |
| Client methods (load, connect, invoke) | `javascript-api k6-net-grpc client` |
| gRPC performance testing guide | `testing-guides performance-testing-grpc-services` |

Key gotchas:
- Proto definitions required: `client.load()` from files OR `client.connect(addr, { reflect: true })`.
- Unary: `client.invoke('package.Service/Method', data)`.
- Streaming: `new Stream(client, ...)` with `on('data')`, `write()`, `end()`.
- Check status with `response.status === grpc.StatusOK`.
