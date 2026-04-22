# specscore-cli

Reference CLI for [SpecScore](https://specscore.md) — lint, query, and scaffold SpecScore specifications.

## Install

```bash
curl -fsSL https://specscore.md/get-cli | sh
```

See [installation docs](https://specscore.md/installation) for options (version pinning, custom install dir).

## Usage

```bash
specscore spec lint              # lint the current spec tree
specscore feature list           # list features
specscore feature show <slug>    # inspect a feature
specscore task list              # show the task board
specscore version                # full build identity
specscore --version              # bare semver
```

Full command reference: see [`spec/features/cli/`](spec/features/cli/).

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Related

- [synchestra-io/specscore](https://github.com/synchestra-io/specscore) — the SpecScore format and documentation
