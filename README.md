# specscore-cli

[![CI](https://github.com/specscore/specscore-cli/actions/workflows/go-ci.yml/badge.svg)](https://github.com/specscore/specscore-cli/actions/workflows/go-ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-99%25-brightgreen)](https://github.com/specscore/specscore-cli/actions/workflows/go-ci.yml)

CLI for [SpecScore](https://specscore.md) — lint, query, and scaffold SpecScore specifications.

## Install

### macOS / Linux — curl

```bash
curl -fsSL https://specscore.md/install/get-cli | sh
```

### Windows — PowerShell

```powershell
powershell -c "irm https://specscore.md/install/get-cli.ps1 | iex"
```

### macOS — Homebrew

```bash
brew install specscore/tap/specscore
```

### Windows — Scoop / WinGet

```powershell
scoop bucket add specscore https://github.com/specscore/scoop-bucket
scoop install specscore
# or
winget install SpecScore.CLI
```

See [installation docs](https://specscore.md/install) for options (version pinning, custom install dir).

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

If you drive `specscore` from inside Claude Code (or any agent host that loads Claude Code plugins), install the [`ai-plugin-specscore`](https://github.com/specscore/ai-plugin-specscore) plugin. It ships agent skills that wrap each CLI resource group — they teach the agent *when* to call which command, *which* flags to pass, and *how* to interpret exit codes, all grounded in the feature specs in this repo.

```
/plugin marketplace add specscore/ai-marketplace
/plugin install specscore@specscore
```

Then bootstrap the CLI itself with `/specscore:install`, or install manually with the one-liner above.

## Test coverage

We're proud that `specscore-cli` maintains **99% test coverage**, enforced automatically — the CI pipeline rejects any pull request that drops below 99%.

Our goal is **100%**. Every uncovered statement is a known gap tracked in [`spec/ideas/full-test-coverage.md`](spec/ideas/full-test-coverage.md).

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Related

- [specscore/specscore](https://github.com/specscore/specscore) — the SpecScore format and documentation
- [specscore/ai-plugin-specscore](https://github.com/specscore/ai-plugin-specscore) — agent skills that wrap this CLI
