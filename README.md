# specscore-cli

CLI for [SpecScore](https://specscore.md) — lint, query, and scaffold SpecScore specifications.

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

## AI skills

If you drive `specscore` from inside Claude Code (or any agent host that loads Claude Code plugins), install the [`ai-plugin-specscore`](https://github.com/synchestra-io/ai-plugin-specscore) plugin. It ships agent skills that wrap each CLI resource group — they teach the agent *when* to call which command, *which* flags to pass, and *how* to interpret exit codes, all grounded in the feature specs in this repo.

```
/plugin marketplace add synchestra-io/ai-marketplace
/plugin install specscore@synchestra-io
```

Then bootstrap the CLI itself with `/specscore:install`, or install manually with the one-liner above.

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Related

- [synchestra-io/specscore](https://github.com/synchestra-io/specscore) — the SpecScore format and documentation
- [synchestra-io/ai-plugin-specscore](https://github.com/synchestra-io/ai-plugin-specscore) — agent skills that wrap this CLI
