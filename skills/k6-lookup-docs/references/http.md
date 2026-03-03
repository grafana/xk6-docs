# k6/http

| I need to... | Path |
|---|---|
| All HTTP methods and classes | `javascript-api k6-http` |
| A specific method (get, post, del, etc.) | `javascript-api k6-http <method>` |
| Response object (status, body, timings, json()) | `javascript-api k6-http response` |
| Params (headers, cookies, timeouts, tags) | `javascript-api k6-http params` |
| Send multiple requests in parallel | `javascript-api k6-http batch` |
| Redirects, URL grouping, request tags | `using-k6 http-requests` |
| Cookies | `using-k6 cookies` |
| Debug HTTP requests/responses | `using-k6 http-debugging` |
| Full CRUD example | `examples api-crud-operations` |
| Authentication example | `examples http-authentication` |
| File uploads example | `examples data-uploads` |

Key gotchas:
- POST/PUT bodies are strings — `JSON.stringify(obj)` + `Content-Type` header.
- `res.json()` caches. `res.json('field')` extracts a field.
- Dynamic URL paths create unique metric names — use `tags: { name: 'Group' }` or `` http.url`...` `` to group.
- Data from `setup()` passed to `default(data)` must be JSON-serializable (no functions).
- `check()` does NOT abort on failure. Combine with thresholds. See [test-config.md](test-config.md).
