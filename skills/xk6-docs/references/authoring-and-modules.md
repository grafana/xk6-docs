# Test Authoring, Modules, and Extensions

## Test Authoring

| I need to... | Path |
|---|---|
| Create a test script using the CLI (`k6 new`) | `using-k6 test-authoring create-test-script-using-the-cli` |
| Generate a test from an OpenAPI spec | `using-k6 test-authoring create-test-script-using-openapi` |
| Use the test builder | `using-k6 test-authoring test-builder` |
| Create tests from browser recordings/HAR files | `using-k6 test-authoring create-tests-from-recordings` |
| Manage dependencies (`k6 deps`) | `using-k6 k6-deps-command` |

## Modules and Imports

| I need to... | Path |
|---|---|
| How modules work (built-in, local, remote, extension) | `using-k6 modules` |
| JavaScript/TypeScript compatibility | `using-k6 javascript-typescript-compatibility-mode` |
| Bundling and transpiling | `examples bundling-and-transpiling` |
| `open()` for reading local files in init | `javascript-api init-context` |
| `import.meta.resolve` | `javascript-api import.meta` |

Key gotchas:
- k6 uses browser-like module resolution — file names must be fully specified (e.g. `./helpers.js`, not `./helpers`).
- `open()` only works in init context (top-level), not inside `default` or `setup`.
- Remote modules (jslib) are imported via full URL: `import { ... } from 'https://jslib.k6.io/...'`.
- Extension modules (`k6/x/*`) are auto-resolved on import — no manual build needed.

## Fault Injection

| I need to... | Path |
|---|---|
| xk6-disruptor overview | `testing-guides injecting-faults-with-xk6-disruptor` |
