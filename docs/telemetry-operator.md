# SpecScore CLI Telemetry — Operator Runbook

> Intended audience: SpecScore maintainers provisioning + verifying telemetry
> infrastructure. End users should read [`docs/telemetry.md`](telemetry.md).

This document tracks the operational steps that satisfy the
[`cli/telemetry/usage-telemetry`](https://specscore.md/cli/telemetry) Feature's
operational acceptance criteria. The code-level wiring is automated; these
steps require human action in the PostHog UI and GitHub repo settings.

---

## 1. PostHog project provisioning

**One-time setup** (per
`cli/telemetry/usage-telemetry#ac:posthog-funnel-defined`).

1. **Create the PostHog project**
   - Region: **EU** (https://eu.posthog.com).
   - Name: **`specscore-cli`**.
   - The region MUST be EU — the code constant `posthogEUEndpoint`
     hard-codes `https://eu.i.posthog.com` (see
     `internal/telemetry/usage.go`); a US project will not receive events.

2. **Capture the project API key (write key)**
   - PostHog settings → Project → Project API Key.
   - This is the *public* write key, not a personal access token.

3. **Store as a GitHub Actions secret**
   - Repo settings → Secrets and variables → Actions → New repository
     secret.
   - Name: **`POSTHOG_WRITE_KEY`** (exact case — `.goreleaser.yml`
     references it).
   - Value: the project API key from step 2.

4. **Configure the north-star funnel**
   - PostHog → Insights → New insight → Funnel.
   - Name: exactly **`North-Star: First Real Spec Use within 7 Days`**
     (the AC asserts this literal name).
   - Three steps:
     1. `cli.command.completed` where `is_first_run = true`
     2. `cli.command.completed` where `command = feature.create`
     3. `cli.command.completed` where `command = feature.create`,
        constrained to "within 7 days of step 1"
   - Group by **`distinct_id`** (which equals `install_id` per
     `REQ:usage-stats-event-properties`).

5. **Record in the release notes**
   - The v0.2.0 release notes MUST include a one-line confirmation that
     the project + funnel were provisioned and the secret is in place.
   - Suggested form: "Telemetry verified: PostHog project `specscore-cli`
     (EU) configured; funnel `North-Star: First Real Spec Use within
     7 Days` present; `POSTHOG_WRITE_KEY` secret set in GitHub Actions."

---

## 2. Verification after each release

After a release ships:

1. **Confirm events are arriving**: PostHog → Activity → Live events. Run
   `specscore --version` locally with a release binary; an event should
   appear within ~30 seconds.
2. **Confirm the funnel is collecting**: PostHog → the named funnel.
   First-run events should accumulate as users install.
3. **Confirm dev builds DO NOT transmit**: build with `goreleaser build
   --snapshot` (no secret in env), run `specscore --version`, verify no
   event arrives in PostHog (the write-key-empty no-op path).

---

## 3. Quota monitoring

PostHog free tier allows 1M events/month. The Idea's assumption was that
Q2 volume (≈260k events at 100 retained users) sits comfortably below
the cap. If usage exceeds 50% of quota in any month, file a follow-up
Idea to either upgrade the plan or introduce sampling.

---

## 4. Disable / remove the project

If the project is ever decommissioned:

1. Remove `POSTHOG_WRITE_KEY` from GitHub Actions secrets.
2. Release a new version: the empty key path causes channels to
   register but transmit-fns to no-op.
3. Delete the PostHog project (no events will arrive thereafter).

The CLI itself does not need code changes — the no-op path was designed
for exactly this case.

---

## Related documents

- User-facing telemetry doc: [`docs/telemetry.md`](telemetry.md)
- Feature contract: <https://specscore.md/cli/telemetry>
- Operational AC: `cli/telemetry/usage-telemetry#ac:posthog-funnel-defined`
