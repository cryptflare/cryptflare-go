<p align="center">
  <a href="https://cryptflare.com">
    <img src="https://assets.cryptflare.com/logo.png" alt="CryptFlare" width="160" />
  </a>
</p>

<h1 align="center">cryptflare-go</h1>

<p align="center">
  Official Go SDK for the <a href="https://cryptflare.com">CryptFlare</a> API.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/cryptflare/cryptflare-go"><img src="https://pkg.go.dev/badge/github.com/cryptflare/cryptflare-go.svg" alt="Go reference" /></a>
  <a href="https://goreportcard.com/report/github.com/cryptflare/cryptflare-go"><img src="https://goreportcard.com/badge/github.com/cryptflare/cryptflare-go" alt="Go report card" /></a>
</p>

---

## Status

`v0.1.0-alpha` - early development. Wire-level contract matches the [SDK umbrella standards](../../docs/standards/sdk/index.md). Currently exposes a typed `Secrets` resource end-to-end as the template; the remaining 38 resources land incrementally and follow the same shape. The Go SDK and TypeScript SDK ship method-parity per the umbrella before either reaches `1.0`.

## Install

```bash
go get github.com/cryptflare/cryptflare-go@latest
```

Module declares `go 1.22`; the SDK is tested on Go 1.22 + the latest stable release. Go 1.21 and earlier are refused at the toolchain level.

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/cryptflare/cryptflare-go"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    client, err := cryptflare.NewClient(
        cryptflare.WithAPIKey(os.Getenv("CRYPTFLARE_API_KEY")),
        cryptflare.WithDefaultOrganisation("org_xyz"),
        cryptflare.WithDefaultWorkspace("my-app"),
        cryptflare.WithDefaultEnvironment("production"),
    )
    if err != nil {
        panic(err)
    }

    revealed, err := client.Secrets.Reveal(ctx, &cryptflare.RevealSecretInput{
        Key: "DATABASE_URL",
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(revealed.Value)
}
```

## Authentication

Three credential modes, mutually exclusive:

```go
cryptflare.WithAPIKey(token)         // personal access token (cf_pat_...)
cryptflare.WithServiceToken(token)   // org service token (cf_svc_...)
cryptflare.WithTokenSource(fn)       // refreshable callback
```

Env-var fallback when no option is supplied: `CRYPTFLARE_API_KEY`, then `CRYPTFLARE_SERVICE_TOKEN`. Workers / serverless: pass the option explicitly; the SDK never reads `os.Getenv` from inside hot paths.

## Errors

Every non-2xx response surfaces as an `*APIError`. Sentinel errors per status work with `errors.Is`:

```go
_, err := client.Secrets.Reveal(ctx, in)
switch {
case errors.Is(err, cryptflare.ErrUnauthenticated):
    // refresh credential and retry
case errors.Is(err, cryptflare.ErrNotFound):
    // secret does not exist
case errors.Is(err, cryptflare.ErrRateLimited):
    var apiErr *cryptflare.APIError
    _ = errors.As(err, &apiErr)
    time.Sleep(apiErr.RetryAfter)
}
```

Full payload via `errors.As`:

```go
var apiErr *cryptflare.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.Code, apiErr.RequestID, apiErr.Status)
}
```

Transport failures wrap `*ConnectionError` or `*TimeoutError`. Construction-time validation surfaces as `*ConfigurationError`.

## Pagination

Cursor-based across every list endpoint. Two iteration shapes:

```go
// Manual paging
page, err := client.Secrets.List(ctx, in).First(ctx)
for page != nil {
    for _, s := range page.Data {
        fmt.Println(s.Key)
    }
    page, err = page.Next(ctx)
    if err != nil { return err }
}

// Callback (preferred)
err := client.Secrets.List(ctx, in).Each(ctx, func(item cryptflare.SecretListItem) error {
    fmt.Println(item.Key)
    return nil
})
```

Stop early by returning `cryptflare.ErrStopIteration` from the callback.

## Retries + idempotency

Defaults match the [umbrella spec](../../docs/standards/sdk/index.md): 3 attempts, full jitter, base 500 ms, cap 8 s. Override:

```go
cryptflare.WithRetry(cryptflare.RetryOptions{ MaxAttempts: 5 })
cryptflare.WithoutRetry()
```

Mutating verbs auto-attach `Idempotency-Key` (UUID v4) when retries are enabled. Override per-call:

```go
client.Secrets.Create(ctx, in, cryptflare.WithIdempotencyKey("create-stripe-key-2026"))
```

## Examples

- [`examples/basic`](./examples/basic/main.go) - list and reveal one secret.

## Standards

This SDK follows the [Go SDK standards](../../../docs/standards/sdk/go.md) and the cross-language [SDK umbrella](../../../docs/standards/sdk/index.md). Read those before opening a PR.

## License

Apache-2.0
