// Package resources hosts every resource the Go SDK exposes. Files are
// kept under 300 lines per the Go SDK standards; new methods that push
// the file over budget split into a sibling file with a topic prefix
// (e.g. `secrets_batch.go`).
//
// This file is intentionally excluded from the public package - the
// hand-curated `*Secrets` type lives in the root package and delegates
// here only for shapes that benefit from co-location.
package resources
