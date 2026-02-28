---
title: 'get'
---
## http.get(url, [params])

Make an HTTP GET request.

{{< code >}}
import http from 'k6/http';

export default function () {
  const res = http.get('https://test-api.k6.io/');
}
{{< /code >}}
