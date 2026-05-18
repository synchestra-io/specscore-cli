// Package property parses and represents SpecScore Property artifacts.
//
// A Property is a single markdown file at `spec/features/**/<slug>.property.md`
// whose YAML frontmatter declares a reusable, named business field — its
// `data_type` and a `checks` mapping. See `spec/features/property/README.md`
// in the meta-spec for the full artifact contract and
// `spec/features/cli/property/README.md` in this repo for the CLI's
// implementation contract.
//
// This package provides discovery and parsing utilities only. The lint rules
// that enforce the schema live in `pkg/lint`.
package property
