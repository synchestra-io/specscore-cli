// Package entity parses and represents SpecScore entity artifacts
// (`*.entity.md` files under `spec/features/**`).
//
// The package is a leaf parser: it provides Discover, Parse, and Walk
// over entity files plus a typed Doc representation. Lint rules and CLI
// verbs that consume entities import this package; parsing here never
// imports `pkg/lint` or `internal/cli`.
//
// Parse is intentionally resilient — it returns a partial Doc for
// malformed input so lint can report every issue rather than bailing on
// the first. See `spec/features/cli/entity/README.md` for the validation
// contract.
package entity
