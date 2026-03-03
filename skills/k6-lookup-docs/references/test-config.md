# Test Configuration Workflow

## Lookup Paths

| I need to... | Path |
|---|---|
| Understand the test lifecycle (init → setup → VU → teardown) | `using-k6 test-lifecycle` |
| Validate responses (checks) | `using-k6 checks` |
| Set pass/fail criteria (thresholds) | `using-k6 thresholds` |
| Configure VU scheduling (scenarios and executors) | `using-k6 scenarios` |
| See all executor types | `using-k6 scenarios executors` |
| Look up all k6 options | `using-k6 k6-options` (subtopics: `reference`, `how-to`) |
| Use built-in or custom metrics | `using-k6 metrics` (subtopics: `reference`, `create-custom-metrics`) |
| Tag and group results | `using-k6 tags-and-groups` |
| Use environment variables | `using-k6 environment-variables` |

## Key Things to Know

- **Checks don't fail tests.** `check()` records pass/fail rates but does NOT abort. To fail a test, combine checks with thresholds.
- **Thresholds fail tests.** `thresholds: { http_req_duration: ['p(95)<200'] }` — if the condition is false at test end, k6 exits non-zero.
- **Lifecycle:** `init` (imports, file loading — no HTTP) → `setup()` (runs once, can do HTTP) → `default(data)` (runs per VU per iteration) → `teardown(data)` (runs once).
- **Data passing:** `setup()` return value is passed to `default` and `teardown` — but only JSON-serializable data, no functions. Each VU gets a fresh copy.
- **Executors:** closed model = `*-vus`/`*-iterations` (fixed VUs). Open model = `*-arrival-rate` (fixed request rate, VUs scale).
