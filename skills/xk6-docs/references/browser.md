# k6/browser

| I need to... | Path |
|---|---|
| Complete walkthrough (imports, options, async) | `using-k6-browser write-your-first-browser-test` |
| Interact with page elements | `using-k6-browser how-to-write-browser-tests` |
| Page, Locator, BrowserContext APIs | `javascript-api k6-browser` |
| Dynamic elements, cookie banners, input delays | `using-k6-browser recommended-practices` |
| Migrate from Playwright | `using-k6-browser migrate-from-playwright-to-k6` |
| Playwright API equivalents in k6 | `using-k6-browser playwright-apis-in-k6` |
| Browser-specific metrics | `using-k6-browser metrics` |
| Browser-specific options | `using-k6-browser options` |

Key gotchas:
- Requires `options.scenarios.*.options.browser.type = 'chromium'`.
- The `default` function MUST be `async`. All browser/page/locator calls need `await`.
- Import: `import { browser } from 'k6/browser'`.
- Prefer `page.locator(selector)` (CSS or XPath) over `page.$()`.
- Always `await page.close()` in a `finally` block.

## Hybrid tests (browser + protocol)

Combine browser VUs (few, expensive) with protocol VUs (many, cheap) using separate scenarios.

| I need to... | Path |
|---|---|
| Hybrid approach guide and example | `using-k6-browser recommended-practices hybrid-approach-to-performance` |
| Configure multiple scenarios | `using-k6 scenarios` |

Key gotchas:
- Use separate scenarios: one with `options.browser.type = 'chromium'`, one without.
- Each scenario gets its own `exec` function.
- Set separate thresholds for HTTP metrics vs browser metrics (e.g. `http_req_duration` vs `browser_web_vital_lcp`).
- Browser VUs are resource-heavy — use few (1-5) alongside many protocol VUs.
- Stagger with `startTime` to control when each scenario begins.
