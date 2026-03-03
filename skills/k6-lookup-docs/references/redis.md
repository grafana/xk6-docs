# k6/x/redis

| I need to... | Path |
|---|---|
| Redis client API | `javascript-api k6-x-redis` |
| Client methods | `javascript-api k6-x-redis client` |
| Connection options | `javascript-api k6-x-redis redis-options` |
| Get values | `javascript-api k6-x-redis client get` |
| Set values | `javascript-api k6-x-redis client set` |

Key gotchas:
- Promise-based API — use `await` for all operations.
- Can load-test Redis OR use it as a data store for test logic.
- Run `--depth 1` on `javascript-api k6-x-redis client` to see all 37+ available commands.
- Extension module — auto-resolved on import.
