# k6/crypto

| I need to... | Path |
|---|---|
| Hashing and HMAC API | `javascript-api k6-crypto` |

Functions: `createHash`, `createHmac`, `hmac`, `md5`, `sha256`, `sha512`, `randomBytes`, etc.

Key gotcha: Prefer Web Crypto (`javascript-api crypto`) over `k6/crypto` for new code — standard API with encryption/decryption/signing, not just hashing.

# Web Crypto API

| I need to... | Path |
|---|---|
| Web Crypto API (encrypt, decrypt, sign, verify) | `javascript-api crypto` |
| SubtleCrypto methods | `javascript-api crypto subtlecrypto` |

Functions: `getRandomValues`, `randomUUID`, `subtle` (encrypt/decrypt/sign/verify/digest/deriveKey).
