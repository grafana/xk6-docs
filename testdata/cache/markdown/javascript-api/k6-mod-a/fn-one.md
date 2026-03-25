---
title: 'fn-one'
---
## modA.fnOne(url, [params])

Perform a fn-one operation.

{{< code >}}
import modA from 'k6/mod-a';

export default function () {
  const res = modA.fnOne('https://test.example.com/');
}
{{< /code >}}
