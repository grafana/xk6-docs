# k6 Best Practices

A comprehensive guide to writing effective, maintainable, and performant k6 load tests.

## Test Structure and Organization

### Use the k6 Test Lifecycle

k6 has four distinct lifecycle stages. Use them intentionally:

```javascript
// 1. init — runs once per VU, used to set up test data and imports
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = 'https://test-api.example.com';

export const options = {
  stages: [
    { duration: '1m', target: 20 },
    { duration: '3m', target: 20 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    checks: ['rate>0.99'],
  },
};

// 2. setup — runs once before test, used for auth tokens, seed data, etc.
export function setup() {
  const loginRes = http.post(BASE_URL + '/auth/login', JSON.stringify({
    username: 'testuser',
    password: 'testpass',
  }), { headers: { 'Content-Type': 'application/json' } });

  const token = loginRes.json('token');
  return { token };
}

// 3. default function — runs repeatedly for each VU iteration
export default function (data) {
  const params = {
    headers: { Authorization: 'Bearer ' + data.token },
  };

  const res = http.get(BASE_URL + '/api/items', params);
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response has items': (r) => r.json('items').length > 0,
  });

  sleep(1);
}

// 4. teardown — runs once after test, used for cleanup
export function teardown(data) {
  http.post(BASE_URL + '/auth/logout', null, {
    headers: { Authorization: 'Bearer ' + data.token },
  });
}
```

### Group Related Requests

Use `group()` to organize related requests into logical transaction blocks:

```javascript
import { group, check } from 'k6';
import http from 'k6/http';

export default function () {
  group('user registration flow', function () {
    const signupRes = http.post('https://api.example.com/signup', JSON.stringify({
      email: 'user@example.com',
      password: 'securepass',
    }), { headers: { 'Content-Type': 'application/json' } });
    check(signupRes, { 'signup status 201': (r) => r.status === 201 });

    const verifyRes = http.get('https://api.example.com/verify?token=abc');
    check(verifyRes, { 'verify status 200': (r) => r.status === 200 });
  });

  group('user login flow', function () {
    const loginRes = http.post('https://api.example.com/login', JSON.stringify({
      email: 'user@example.com',
      password: 'securepass',
    }), { headers: { 'Content-Type': 'application/json' } });
    check(loginRes, { 'login status 200': (r) => r.status === 200 });
  });
}
```

## Performance and Resource Management

### Add Think Time Between Requests

Real users do not fire requests instantly. Use `sleep()` to simulate realistic pacing:

```javascript
import { sleep } from 'k6';
import http from 'k6/http';

export default function () {
  http.get('https://test.k6.io/');
  sleep(Math.random() * 3 + 1); // 1-4 seconds of random think time
}
```

### Use Checks Instead of console.log

Avoid `console.log()` for validations — it does not integrate with k6 metrics. Use `check()` instead:

```javascript
import { check } from 'k6';
import http from 'k6/http';

export default function () {
  const res = http.get('https://test.k6.io/');

  // Bad: does not track pass/fail metrics
  // console.log('Status:', res.status);

  // Good: integrates with thresholds and summary
  check(res, {
    'status is 200': (r) => r.status === 200,
    'body is not empty': (r) => r.body.length > 0,
  });
}
```

### Set Thresholds for Pass/Fail Criteria

Define clear thresholds so CI pipelines can gate on performance:

```javascript
export const options = {
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    'http_req_duration{name:login}': ['p(95)<800'],
  },
};
```

## Error Handling Patterns

### Validate Responses Thoroughly

Check more than just the status code:

```javascript
import { check } from 'k6';
import http from 'k6/http';

export default function () {
  const res = http.get('https://api.example.com/users/1');

  check(res, {
    'status is 200': (r) => r.status === 200,
    'content-type is json': (r) => r.headers['Content-Type'].includes('application/json'),
    'body has user id': (r) => r.json('id') !== undefined,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
```

### Handle Expected Errors Gracefully

Not all non-200 responses are failures. Design checks for your actual expectations:

```javascript
import { check } from 'k6';
import http from 'k6/http';

export default function () {
  // This endpoint might return 404 for missing items — that is expected
  const res = http.get('https://api.example.com/items/nonexistent');

  check(res, {
    'returns 404 for missing item': (r) => r.status === 404,
    'error message is descriptive': (r) => r.json('error') !== '',
  });
}
```

## Data Management and Parameterization

### Use SharedArray for Large Datasets

`SharedArray` shares data across VUs, saving memory:

```javascript
import { SharedArray } from 'k6/data';
import http from 'k6/http';

// Data is loaded once and shared across all VUs (read-only)
const users = new SharedArray('users', function () {
  return JSON.parse(open('./data/users.json'));
});

export default function () {
  const user = users[Math.floor(Math.random() * users.length)];
  http.post('https://api.example.com/login', JSON.stringify({
    username: user.username,
    password: user.password,
  }), { headers: { 'Content-Type': 'application/json' } });
}
```

### Parameterize with Environment Variables

Use `__ENV` for runtime configuration:

```javascript
const BASE_URL = __ENV.BASE_URL || 'https://test.k6.io';
const API_KEY = __ENV.API_KEY;

export default function () {
  const res = http.get(BASE_URL + '/api/data', {
    headers: { 'X-API-Key': API_KEY },
  });
}
```

Run with:
```bash
k6 run -e BASE_URL=https://staging.example.com -e API_KEY=secret script.js
```

### Use Execution Context for Unique Data

Avoid data collisions between VUs using the execution API:

```javascript
import exec from 'k6/execution';
import { SharedArray } from 'k6/data';

const users = new SharedArray('users', function () {
  return JSON.parse(open('./data/users.json'));
});

export default function () {
  // Each VU gets a unique user based on its ID
  const user = users[exec.vu.idInTest % users.length];
}
```

## Authentication Strategies

### Authenticate Once in setup()

Do not authenticate on every iteration — authenticate once and pass the token:

```javascript
import http from 'k6/http';

export function setup() {
  const res = http.post('https://api.example.com/auth/token', JSON.stringify({
    client_id: __ENV.CLIENT_ID,
    client_secret: __ENV.CLIENT_SECRET,
  }), { headers: { 'Content-Type': 'application/json' } });

  return { token: res.json('access_token') };
}

export default function (data) {
  http.get('https://api.example.com/protected', {
    headers: { Authorization: 'Bearer ' + data.token },
  });
}
```

### Per-VU Authentication When Needed

If each VU needs its own session, authenticate in the init or default function:

```javascript
import http from 'k6/http';
import exec from 'k6/execution';
import { SharedArray } from 'k6/data';

const credentials = new SharedArray('creds', function () {
  return JSON.parse(open('./data/credentials.json'));
});

export default function () {
  const cred = credentials[exec.vu.idInTest % credentials.length];
  const loginRes = http.post('https://api.example.com/login', JSON.stringify(cred), {
    headers: { 'Content-Type': 'application/json' },
  });
  const token = loginRes.json('token');

  // Use the token for subsequent requests in this iteration
  http.get('https://api.example.com/me', {
    headers: { Authorization: 'Bearer ' + token },
  });
}
```

## Monitoring and Observability

### Use Custom Metrics for Business KPIs

Track domain-specific metrics alongside HTTP metrics:

```javascript
import { Trend, Counter, Rate } from 'k6/metrics';
import http from 'k6/http';
import { check } from 'k6';

const orderDuration = new Trend('order_processing_time');
const orderCount = new Counter('orders_placed');
const orderSuccess = new Rate('order_success_rate');

export default function () {
  const res = http.post('https://api.example.com/orders', JSON.stringify({
    items: [{ id: 1, quantity: 2 }],
  }), { headers: { 'Content-Type': 'application/json' } });

  orderDuration.add(res.timings.duration);
  const success = res.status === 201;
  orderSuccess.add(success);
  if (success) {
    orderCount.add(1);
  }
}
```

### Tag Requests for Granular Analysis

Use tags to break down metrics by endpoint or operation:

```javascript
import http from 'k6/http';

export default function () {
  http.get('https://api.example.com/users', {
    tags: { name: 'GetUsers', type: 'list' },
  });

  http.get('https://api.example.com/users/1', {
    tags: { name: 'GetUser', type: 'detail' },
  });
}
```

## Design Patterns: Ramping, Stages, and Scenarios

### Use Scenarios for Realistic Workloads

Scenarios let you model different traffic patterns running simultaneously:

```javascript
import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  scenarios: {
    browse: {
      executor: 'constant-vus',
      vus: 50,
      duration: '5m',
      exec: 'browsePage',
    },
    checkout: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 20,
      maxVUs: 100,
      stages: [
        { duration: '2m', target: 10 },
        { duration: '3m', target: 10 },
        { duration: '1m', target: 0 },
      ],
      exec: 'checkoutFlow',
    },
  },
};

export function browsePage() {
  http.get('https://ecommerce.example.com/');
  sleep(2);
}

export function checkoutFlow() {
  http.post('https://ecommerce.example.com/cart/add', JSON.stringify({ item: 1 }), {
    headers: { 'Content-Type': 'application/json' },
  });
  sleep(1);
  http.post('https://ecommerce.example.com/checkout');
}
```

### Choose the Right Executor

| Executor | Use case |
|----------|----------|
| `shared-iterations` | Fixed total iterations split across VUs — good for one-time batch jobs |
| `per-vu-iterations` | Each VU runs a fixed number of iterations — good for per-user workflows |
| `constant-vus` | Constant number of VUs — simplest steady-state test |
| `ramping-vus` | Ramp VUs up/down — classic load profile |
| `constant-arrival-rate` | Fixed request rate regardless of response time — good for SLO testing |
| `ramping-arrival-rate` | Ramp request rate up/down — find breaking points |

### Ramp Up and Down Gracefully

Avoid slamming your system with full load from the start:

```javascript
export const options = {
  stages: [
    { duration: '2m', target: 50 },   // ramp up
    { duration: '5m', target: 50 },   // steady state
    { duration: '2m', target: 100 },  // push higher
    { duration: '5m', target: 100 },  // steady at peak
    { duration: '3m', target: 0 },    // ramp down
  ],
};
```

## Code Quality: Modules and Shared Code

### Extract Reusable Code into Modules

Keep test scripts clean by extracting helpers:

```javascript
// helpers/api.js
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://api.example.com';

export function apiGet(path, token) {
  const res = http.get(BASE_URL + path, {
    headers: { Authorization: 'Bearer ' + token },
  });
  return res;
}

export function apiPost(path, body, token) {
  const res = http.post(BASE_URL + path, JSON.stringify(body), {
    headers: {
      'Content-Type': 'application/json',
      Authorization: 'Bearer ' + token,
    },
  });
  return res;
}

export function checkStatus(res, expectedStatus, name) {
  check(res, {
    [name || 'status is ' + expectedStatus]: (r) => r.status === expectedStatus,
  });
}
```

```javascript
// test.js — uses the helper module
import { apiGet, apiPost, checkStatus } from './helpers/api.js';

export function setup() {
  const res = apiPost('/auth/login', { user: 'admin', pass: 'admin' });
  return { token: res.json('token') };
}

export default function (data) {
  const res = apiGet('/users', data.token);
  checkStatus(res, 200, 'get users');
}
```

### Use options Exports for Configuration

Keep configuration in the test script (or import from a shared config):

```javascript
// config/load-profile.js
export const smokeTest = {
  vus: 1,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<500'],
  },
};

export const loadTest = {
  stages: [
    { duration: '5m', target: 100 },
    { duration: '10m', target: 100 },
    { duration: '5m', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<800', 'p(99)<1500'],
    http_req_failed: ['rate<0.01'],
  },
};
```

## Browser Testing Best Practices

### Use k6 Browser for End-to-End Testing

Combine protocol-level and browser-level testing:

```javascript
import { browser } from 'k6/browser';
import { check } from 'k6';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
          type: 'chromium',
        },
      },
    },
  },
};

export default async function () {
  const page = await browser.newPage();

  try {
    await page.goto('https://test.k6.io/');

    const header = await page.locator('h1');
    check(await header.textContent(), {
      'header is correct': (text) => text.includes('Welcome'),
    });

    await page.locator('a[href="/contacts.php"]').click();
    await page.waitForNavigation();

    check(page, {
      'navigated to contacts': (p) => p.url().includes('/contacts'),
    });
  } finally {
    await page.close();
  }
}
```

### Mix Browser and Protocol Tests

Run browser tests alongside API tests for comprehensive coverage:

```javascript
import { browser } from 'k6/browser';
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    api_load: {
      executor: 'constant-vus',
      vus: 50,
      duration: '5m',
      exec: 'apiTest',
    },
    browser_flow: {
      executor: 'constant-vus',
      vus: 2,
      duration: '5m',
      exec: 'browserTest',
      options: {
        browser: {
          type: 'chromium',
        },
      },
    },
  },
};

export function apiTest() {
  const res = http.get('https://test.k6.io/api/data');
  check(res, { 'api status 200': (r) => r.status === 200 });
  sleep(1);
}

export async function browserTest() {
  const page = await browser.newPage();
  try {
    await page.goto('https://test.k6.io/');
    check(page, {
      'page loaded': (p) => p.url() === 'https://test.k6.io/',
    });
    sleep(3);
  } finally {
    await page.close();
  }
}
```

### Keep Browser VU Counts Low

Browser tests consume significantly more resources than protocol tests. Use 2-5 browser VUs for realistic frontend testing, and rely on protocol-level VUs for load generation:

```javascript
export const options = {
  scenarios: {
    // Heavy load via protocol
    protocol: {
      executor: 'ramping-vus',
      stages: [
        { duration: '5m', target: 200 },
        { duration: '10m', target: 200 },
        { duration: '5m', target: 0 },
      ],
      exec: 'protocolTest',
    },
    // Light browser presence for Web Vitals and UX metrics
    browser: {
      executor: 'constant-vus',
      vus: 3,
      duration: '20m',
      exec: 'browserTest',
      options: { browser: { type: 'chromium' } },
    },
  },
};
```

## Summary Checklist

- [ ] Use the k6 lifecycle stages correctly (init, setup, default, teardown)
- [ ] Set meaningful thresholds for CI/CD gating
- [ ] Use checks, not console.log, for validations
- [ ] Add realistic think time with sleep()
- [ ] Use SharedArray for large datasets
- [ ] Tag requests for granular metric analysis
- [ ] Extract reusable code into modules
- [ ] Choose the right executor for your use case
- [ ] Ramp up gradually — do not start at peak load
- [ ] Keep browser VUs low, use protocol VUs for load
- [ ] Use custom metrics for business-specific KPIs
- [ ] Authenticate once in setup() when possible
- [ ] Parameterize environment-specific values with __ENV
