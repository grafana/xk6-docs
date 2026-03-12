# TLS

## k6/x/tls (certificate inspection)

| I need to... | Path |
|---|---|
| TLS certificate inspection API | `javascript-api k6-x-tls` |

## SSL/TLS protocol configuration

| I need to... | Path |
|---|---|
| SSL/TLS overview | `using-k6 protocols ssl-tls` |
| Client certificates | `using-k6 protocols ssl-tls client-certificates` |
| TLS version and cipher config | `using-k6 protocols ssl-tls version-and-ciphers` |
| OCSP stapling | `using-k6 protocols ssl-tls online-certificate-status-protocol-ocsp` |

Key gotcha: `k6/x/tls` is for **inspecting** remote certificates (read-only). `using-k6 protocols ssl-tls` is for **configuring** k6's TLS client behavior (client certs, ciphers, versions).
