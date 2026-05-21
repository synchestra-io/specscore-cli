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

## 1b. Sentry project provisioning

**One-time setup** (per
`cli/telemetry/errors-telemetry#ac:sentry-alert-on-new-signature`).

1. **Create the Sentry project**
   - Region: **EU** (https://sentry.io/welcome/eu/).
   - Name: **`specscore-cli`**.
   - Platform: **Go**.

2. **Capture the project DSN**
   - Sentry → Project settings → Client Keys (DSN). Copy the DSN.
   - It MUST be an EU DSN — the host should look like
     `<key>@<org>.ingest.de.sentry.io`. The code does not validate this
     at runtime; an inadvertent US DSN would route crash reports through
     US infrastructure.

3. **Store as a GitHub Actions secret**
   - Repo settings → Secrets and variables → Actions → New repository
     secret.
   - Name: **`SENTRY_DSN`**.
   - Value: the DSN from step 2.

4. **Configure the new-signature alert rule**
   - Sentry → Alerts → Create alert rule.
   - Trigger: "a new issue is created" (Sentry's term for a new crash
     signature).
   - Filter: `release equals {latest_release}` AND `debug NOT equal "true"`
     (the latter excludes `specscore debug error` invocations from
     paging the founder).
   - Action: send email to the maintainer's address. Slack DM or
     webhook MAY be substituted later.
   - Save the alert-rule permalink (Sentry exposes one per rule).

5. **Record in the release notes**
   - The v0.2.0 release notes MUST include the alert-rule permalink
     AND quote the filter expression including the `debug != "true"`
     exclusion clause, per the Plan's audit-trail requirement.
   - Suggested form: "Sentry alert configured: <permalink> with filter
     `release equals {latest_release} and debug not equal 'true'`."

---

## 2. Verification after each release

After a release ships:

1. **Confirm events are arriving (usage-stats)**: PostHog → Activity → Live
   events. Run `specscore --version` locally with a release binary; an event
   should appear within ~30 seconds.
2. **Confirm the funnel is collecting**: PostHog → the named funnel.
   First-run events should accumulate as users install.
3. **Confirm error pipeline (crash-reports)**: run
   `specscore debug error --text "release-verification" --force` on a
   release binary; a Sentry event tagged `debug=true` should appear in the
   Sentry UI's Issues list within ~30 seconds. The configured alert rule
   should NOT fire (filter excludes `debug=true`).
4. **Confirm dev builds DO NOT transmit**: build with `goreleaser build
   --snapshot` (no secrets in env), run `specscore --version` and
   `specscore debug error --text foo --force`, verify no events arrive in
   either PostHog or Sentry (the empty-key/DSN no-op paths).

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
