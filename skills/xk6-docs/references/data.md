# k6/data

| I need to... | Path |
|---|---|
| SharedArray API | `javascript-api k6-data` |
| SharedArray details | `javascript-api k6-data sharedarray` |

Key gotcha: `SharedArray` is initialized once in init context and shared read-only across all VUs — much more memory-efficient than regular arrays for large datasets.
