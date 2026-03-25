---
title: 'fnOne( url, [params] )'
description: 'Issue a first function call.'
weight: 10
---

# fnOne( url, [params] )

Make a first function call.

See the [API docs](https://grafana.com/docs/k6/<K6_VERSION>/javascript-api/k6-mod-a/fn-one).

{{< code >}}

```javascript
import modA from 'k6/mod-a';

export default function () {
  const res = modA.fnOne('https://test.example.com');
}
```

{{< /code >}}
