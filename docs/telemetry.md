# SpecScore CLI Telemetry

> Canonical reference: <https://specscore.md/cli/telemetry>

This document describes what the `specscore` CLI sends to external services,
where it goes, what it does not collect, and how to disable it.

## Overview

`specscore` collects two distinct kinds of anonymous data so the maintainers
can find bugs and learn what is used:

- **`usage-stats`** — product analytics. One event per CLI invocation
  describing *what command ran* and *whether it succeeded*.
- **`crash-reports`** — error reporting. One event when the CLI panics or
  exits with an unexpected error code (≥10), describing *what broke* with a
  scrubbed stack trace.

Both channels are **opt-out**, **EU-hosted**, and **independent** — you can
disable one without disabling the other.

## Channels

### usage-stats

<!-- Populated by the cli/telemetry/usage-telemetry child Feature.
     Per cli/telemetry/usage-telemetry#req:docs-usage-stats-section, this
     subsection MUST describe the channel, name the PostHog event
     (cli.command.completed), link to PostHog's privacy page, and call out
     the EU region (eu.i.posthog.com). To be filled in when the channel
     ships in v0.2.0. -->

To be populated when the `usage-stats` channel ships (PostHog event
`cli.command.completed`, EU region `eu.i.posthog.com`).

### crash-reports

<!-- Populated by the cli/telemetry/errors-telemetry child Feature.
     Per cli/telemetry/errors-telemetry#req:docs-crash-reports-section,
     this subsection MUST describe the channel, explicitly state that
     only panics and exit codes ≥10 trigger transmission, link to
     Sentry's privacy page, and call out the EU region (de.sentry.io). -->

To be populated when the `crash-reports` channel ships (Sentry, EU region
`*.ingest.de.sentry.io`, triggered by panic or exit code ≥10).

## Event Schema

### usage-stats events

<!-- Per cli/telemetry/usage-telemetry#req:docs-usage-stats-section, this
     subsection MUST enumerate all 10 property keys (matching
     REQ:usage-stats-event-properties) AND the full 20-value caller enum
     (matching REQ:caller-enum-known-values) end-to-end. -->

Each `cli.command.completed` event will carry these ten properties (the closed
enum enforced at compile time by `internal/telemetry/telemetry.go`):

| Property | Value source |
|---|---|
| `command` | Cobra command path, dot-separated (e.g. `feature.create`). |
| `success` | `true` iff exit code is `0`. |
| `duration_ms` | Wall-clock duration, integer milliseconds. |
| `exit_code` | Integer exit code returned by the command. |
| `cli_version` | The version string from `specscore --version`. |
| `os` | `runtime.GOOS` (`darwin`, `linux`, `windows`). |
| `arch` | `runtime.GOARCH` (`amd64`, `arm64`, …). |
| `caller` | The AI-agent identifier (see enum below). |
| `install_id` | Per-machine UUID v4 (also used as PostHog `distinct_id`). |
| `is_first_run` | `true` iff the install_id file was just created. |

The `caller` field is a closed enum populated by the agent driving the CLI
(or `cli` by default for human-typed invocations):

| Value | Agent |
|---|---|
| `cli` | Default — human at a terminal, no agent set. |
| `claude` | Claude Code (Anthropic). Includes invocations via `specstudio-skills`. |
| `codex` | Codex CLI (OpenAI). |
| `aider` | Aider — open-source AI pair programmer. |
| `opencode` | OpenCode — open-source AI coding agent. |
| `goose` | Goose (Block / Square). |
| `cursor` | Cursor (Anysphere). |
| `gemini` | Gemini CLI (Google). |
| `copilot` | GitHub Copilot CLI. |
| `devin` | Devin (Cognition Labs). |
| `cline` | Cline — open-source VS Code AI agent. |
| `roo` | Roo Code — Cline fork. |
| `continue` | Continue — open-source VS Code / JetBrains AI assistant. |
| `windsurf` | Windsurf — Codeium's AI-native IDE. |
| `zed` | Zed — collaborative editor with AI mode. |
| `amazon-q` | Amazon Q Developer (AWS). |
| `tabnine` | Tabnine. |
| `pi.dev` | Pi (https://pi.dev/). |
| `antigravity.google` | Antigravity (https://antigravity.google/). |
| `other` | Catch-all for any unrecognized value. |

To set a caller, pass `--caller <value>` to any `specscore` invocation, or set
`SPECSCORE_CALLER=<value>` in the environment. The flag takes precedence over
the env var. Unrecognized values are coerced to `other` before transmission
— the flag itself accepts any string so scripts cannot accidentally fail.

### crash-reports events

To be populated when the `crash-reports` channel ships. Will document:
- The Sentry event shape (`message` field, scrubbed stack frames, `release`
  and `debug` tags).
- The `SafePanic` allowlist mechanism and the `unscrubbed panic` coercion.
- The `--text` interpretation contract of `specscore debug error`.

## What We Don't Collect

The `internal/telemetry/` Go package is the **only** place in the codebase
where data may leave the binary. A `_test.go`-based boundary check fails the
build if any other file imports a telemetry vendor SDK. Independently, the
public `telemetry.Emit(ctx, Event)` function accepts only a fixed-field
struct — `map[string]any` is a compile-time type error.

This means the following will **never** appear in any telemetry event:

- **Spec content** — the body of any `.md` file in your spec tree.
- **Feature names** — the slugs in `spec/features/<slug>/` (the `command`
  property carries the cobra command, e.g. `feature.create`, NOT the feature
  slug being acted upon).
- **File paths** — absolute paths to your projects (the crash-reports
  scrubber replaces every frame's file path with just its basename).
- **Project paths** — your `cwd`, your `$HOME`, your project directory.
- **Git remotes** — `git remote get-url origin` is never read for telemetry.
- **Hostnames** — your machine's hostname is never read.
- **Usernames** — `os.User`, `git config user.name`, `$USER` are never read.
- **Arbitrary user input** — any string you typed as a command argument
  (the `command` property is the cobra subcommand path, NOT the args you
  passed).
- **Environment variables** other than the recognized opt-out markers
  (`SPECSCORE_TELEMETRY`, `DO_NOT_TRACK`, `CI`, etc., and only their
  boolean evaluation — not their values).

If you find a counter-example, please file an issue.

## Opt-out

Telemetry is **opt-out**. You can disable it in several ways, evaluated in
this order (first positive signal wins):

1. **One-shot flag on a single invocation:**
   ```
   specscore --no-telemetry <subcommand>
   ```

2. **Environment variable for a session, script, or shell profile:**
   ```
   export SPECSCORE_TELEMETRY=0
   # or the rfc-ish convention:
   export DO_NOT_TRACK=1
   ```

3. **Continuous-integration auto-disable** (no action needed — happens
   automatically when `CI=true`, `GITHUB_ACTIONS=true`, `GITLAB_CI=true`,
   `BUILDKITE=true`, or `CIRCLECI=true`).

4. **Persistent preference** (written to `~/.specscore/telemetry.yaml`):
   ```
   specscore telemetry disable                  # global, both channels
   specscore telemetry disable usage-stats      # only product analytics
   specscore telemetry disable crash-reports    # only error reporting
   specscore telemetry enable                   # symmetric for re-enabling
   specscore telemetry status                   # show current state
   ```

A local opt-out always wins over any caller-tagging value — a plugin or
script setting `SPECSCORE_CALLER` cannot bypass your opt-out.

## Data Retention

### usage-stats

To be populated when the channel ships (PostHog's default retention applies
— currently 7 years on the free tier; GDPR deletion endpoint will be
documented here with the `install_id` as the deletion key).

### crash-reports

To be populated when the channel ships (Sentry free-tier retention: 30 days
for individual events, 90 days for issues / crash signatures).

---

For the architectural contract behind these guarantees, see
<https://specscore.md/cli/telemetry>.
