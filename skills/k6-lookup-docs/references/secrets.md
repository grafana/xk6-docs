# k6/secrets

| I need to... | Path |
|---|---|
| Access secrets from configured sources | `javascript-api k6-secrets` |
| Secret source configuration | `using-k6 secret-source` |
| Configure file source | `using-k6 secret-source file` |
| Configure mock source | `using-k6 secret-source mock` |
| Configure URL source | `using-k6 secret-source url` |

Key gotcha: `secrets.get()` is async — use `await` inside an `async` default function.
