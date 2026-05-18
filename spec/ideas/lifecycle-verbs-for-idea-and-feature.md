# Idea: Lifecycle Verbs for Idea and Feature

**Status:** Approved
**Date:** 2026-05-18
**Owner:** alexander.trakhimenok
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let the specscore CLI mutate the lifecycle Status of Idea and Feature artifacts — with validated state-machine transitions and atomic index/lint synchronization — so spec authors and downstream tooling can advance state programmatically without hand-editing Markdown files, while staying out of Synchestra's lane on doc kinds where coordination is the value?

## Context

Today the `specscore` CLI is read-only after `<kind> new` for every document kind. The command tree (per `specscore --help` v0.17.0) exposes `feature`, `idea`, `task`, `code`, `spec`, and `init` — and every mutation surface is creation-only. Transitioning an Idea `Draft → Approved` is a hand-edit of the `**Status:**` line followed by `specscore spec lint --fix` to sync the ideas-index row. There is no command to validate that the transition is legal, no shared state-machine package, and no way for downstream tooling (CI, agent skills, web dashboards) to advance state programmatically.

The CLI's own task command spec ([`spec/features/cli/task/README.md:41`](../features/cli/task/README.md)) names this gap directly: *"transition semantics (who can claim, how conflicts are resolved, when status becomes terminal) warrant their own feature spec."* Its Outstanding Questions list asks: *"When should lifecycle commands (`task claim`, `task release`, `task status`) land?"* This Idea is the architectural answer to that question — but only for Idea and Feature; Task is deliberately deferred per the positioning below.

[Synchestra](https://github.com/synchestra-io/synchestra), a sibling product layered on the SpecScore standard, **already ships rich task lifecycle** at `synchestra task {new,enqueue,claim,start,status,complete,fail,block,unblock,release,abort,aborted}` — twelve verbs backed by a `--sync` policy (`on_commit` / `manual` / `on_session_end` / `on_interval`), a dedicated exit code `1` for conflict (*"another agent claimed first"*), and an `abort_requested` flag distinct from terminal status. Synchestra does **not** currently expose lifecycle verbs for Idea or Feature. This leaves Idea and Feature lifecycle as clean greenfield for `specscore`, and Task as Synchestra's domain by design.

## Recommended Direction

Ship **sugared per-transition verbs** for `idea` and `feature` on the `specscore` CLI, mirroring the vocabulary pattern Synchestra uses for tasks. Each verb validates the source `Status`, mutates the file atomically, and invokes `spec lint --fix` to keep the corresponding index row in sync. State-machine logic lives in a shared internal package (`pkg/lifecycle/`) so the legal-transition graph is defined once and reused by every transition verb.

**Idea verbs:** `specscore idea approve <slug>` (`Draft → Approved`), `specscore idea archive <slug>` (any non-terminal → `Archived`, moving the file under `spec/ideas/archived/` per the existing archived-order lint behavior). The `Specified` transition stays Synchestra-managed (it fires when a Feature declares `source_idea: <id>`). The `Implementing` transition is set externally by plan/task tooling when work begins; no user-facing lifecycle verb in MVP. `idea reopen` (`Approved → Draft`) is **deferred** unless a real reuse pattern surfaces.

**Feature verbs:** `specscore feature review <id>` (`Draft → Under Review`), `specscore feature approve <id>` (`Draft` or `Under Review → Approved`), `specscore feature implement <id>` (`Approved → Implementing`), `specscore feature stabilize <id>` (`Implementing → Stable`), `specscore feature deprecate <id>` (`Stable → Deprecated`). The inverse `feature undeprecate` is **deferred**.

**State-machine validation.** Every verb enforces source-status preconditions. Illegal transitions exit with a dedicated non-zero code (proposed: `4` `InvalidTransition`, conforming to the CLI's [shared exit-code contract](../features/cli/README.md#shared-exit-code-contract)) and a human-readable error naming both the current status and the legal next states. Index/lint sync is part of every transition — partial state (Idea file says `Approved`, index says `Draft`) MUST NOT be observable after a successful command.

**Architectural positioning** (explicit, not implicit): `specscore` ships local-file mutation primitives with state-machine validation. **Synchestra** layers concurrency, sync policies, claim/release semantics, and multi-agent coordination on top. For doc kinds where local-file mutation **is** the value (Idea, Feature) — transitions are deliberate, single-actor, contention-free — `specscore` is the canonical surface. For doc kinds where coordination **is** the value (Task) — concurrency, claim races, abort flags — Synchestra is the canonical surface. `specscore` MAY later mirror a thin task-status primitive as a standalone-OSS fallback for users without Synchestra, but that is a separate Idea whose answer hinges on the standalone-OSS-user use case, not on file mutation.

**Plugin-skill behavior (downstream).** When the corresponding skill in [`ai-plugin-specscore`](https://github.com/synchestra-io/ai-plugin-specscore) is updated to wrap a lifecycle verb, the skill MUST include a pre-flight check: if both `specscore` and a corresponding Synchestra command (e.g., `synchestra task status` once a future Idea-or-Feature equivalent exists) are installed on the user's machine, the skill SHOULD prefer the Synchestra command for that doc kind. This routes richer-context users through Synchestra while keeping standalone-OSS users fully served by `specscore`. For Idea and Feature today, no Synchestra equivalent exists and `specscore` is always the canonical path. This constraint is captured in the Idea so it isn't lost when the plugin-update follow-on lands.

## Alternatives Considered

**Generic primitive `specscore patch <kind> <id> --status <X>`** — one command per kind, transition validated server-side. Rejected because (a) the CLI's existing one-verb-per-file-spec convention biases against generic primitives; (b) discoverability suffers — no `idea approve` appears in `--help` to telegraph the lifecycle; (c) sugared verbs match Synchestra's task vocabulary closely enough that mental-model transfer across the two tools is preserved.

**Hybrid (primitive is canonical, sugared verbs are aliases)** — both surfaces ship; `specscore idea status <slug> Approved` is canonical and `specscore idea approve <slug>` is a thin wrapper. Rejected as **redundant for MVP**: two ways to do the same thing forces docs to explain both, doubles the test matrix, and the discoverability win of sugared verbs already exists. Keep simple. Revisit only if a future doc kind has so many transitions that per-verb specs become unwieldy.

**Synchestra-only ownership (specscore-cli stays read-only)** — defer all lifecycle to Synchestra. Rejected because `specscore`'s standalone-OSS positioning requires it to be functionally complete on its own for the doc kinds where coordination isn't the bottleneck. Forcing OSS-only users to either install Synchestra or hand-edit `**Status:**` lines is a usability regression dressed as architectural purity.

**Build Synchestra event emission into transition commands from day one** — every transition POSTs to a webhook or writes to an event log. Rejected for MVP: event transport, schema, retry semantics, and authentication are real cross-cutting concerns that deserve a dedicated Idea (see Not Doing). Shipping events poorly is worse than not shipping them.

## MVP Scope

One cycle. Implement the verb set above for Idea (`approve`, `archive`) and Feature (`review`, `approve`, `implement`, `stabilize`, `deprecate`). Ship a shared `pkg/lifecycle/` package that encodes the legal-transition graph per doc kind and is consumed by both cobra command files. Each verb runs `spec lint --fix` after the file mutation so the corresponding index row stays consistent. Exit code `4` (`InvalidTransition`) wired through and documented in the CLI shared exit-code contract. Per-verb feature specs under `spec/features/cli/idea/<verb>/` and `spec/features/cli/feature/<verb>/`, each with a worked example. Unit tests cover the full legal-transition matrix and every illegal-transition rejection. Smoke test: re-run the SpecScore meta-spec's Idea transitions (e.g., transition `entity-and-property-definitions` from `Approved` through to a later state once the spec/state model supports it) using only the new verbs, no hand-edits. Out of MVP: Task lifecycle, event emission, owner mutation, `--reason` flags, plugin-skill updates, batch transitions.

## Not Doing (and Why)

- Task lifecycle in `specscore` — Synchestra already ships twelve task verbs with sync policy, claim semantics, and conflict-aware exit codes. Duplicating in `specscore` would create two competing surfaces. Whether `specscore` should later mirror a thin `task status` primitive for standalone-OSS users is its own Idea.
- Owner mutation / assignment verbs — different shape (field overwrite, not state-machine transition). Belongs in a separate `field-mutation` Idea if a real need surfaces.
- Synchestra event emission — `idea.approved`, `feature.shipped`, etc. Out of scope: event transport, schema design, retry semantics, and auth are cross-cutting concerns that need a dedicated Idea.
- Plugin (`ai-plugin-specscore`) skill updates — deferred. Once the CLI ships, the plugin update is mechanical (add `references/approve.md` per the existing `feature/references/info.md` pattern, update the SKILL.md verb table, add the Synchestra-pre-flight detection rule). Separate follow-on Idea or a docs-only PR.
- `--reason` / `--message` audit-trail flags on transitions — deferred. The git commit produced by the user after the transition is the audit trail for MVP. Revisit if multi-actor workflows demand structured reason capture.
- Concurrency, locking, sync policies — by design, out of scope. These are Synchestra's domain. `specscore` assumes single-actor file mutation.
- Programmatic transitions triggered from the lint engine — keep state changes user-initiated for MVP. Auto-transitions (e.g., "lint detects all sub-tasks completed → auto-stabilize feature") are tempting but mask author intent.
- Batch operations (`specscore idea approve <slug-1> <slug-2> ...`) — single-id MVP. Useful at planning time but adds complexity (partial-failure semantics, atomic rollback) that doesn't earn its weight until usage proves it.
- `idea reopen` (`Approved → Draft`) and `feature undeprecate` (`Deprecated → Stable`) — both real but rare. Defer until asked.

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | Idea and Feature transitions are genuinely contention-free in real usage — single-actor, deliberate, no claim races analogous to those Synchestra solves for Tasks. | Survey actual workflows on 2–3 real consumer repos over the first month post-ship. Acceptance: zero reports of two actors racing on the same Idea/Feature transition; if any surface, escalate to a Synchestra-integration Idea. |
| Must-be-true | The legal-transition graph for Idea and Feature can be defined as a directed graph in code without controversy — a single canonical set of transitions is agreed upon. | Enumerate every legal transition before implementation; capture in a `pkg/lifecycle/states.go` table; review the matrix as a standalone PR; lock into a unit-test matrix that exhaustively covers `(from, verb, to)` triples. |
| Must-be-true | Running `spec lint --fix` after every transition stays fast enough not to harm UX on representative repos. | Benchmark on a 200-feature repo and a 50-Idea repo. Acceptance: every transition command (mutate + lint --fix) completes in under 250ms on a developer laptop; if it exceeds, the lint scope is narrowed to the affected index rather than full repo. |
| Should-be-true | Sugared verb naming aligns closely enough with Synchestra's task vocabulary that users moving between the two tools have low cognitive overhead. | Review the final verb list against `synchestra task {claim,start,status,complete,fail,block,unblock,release,abort,aborted}` before shipping. Document any divergence and the reason in `spec/features/cli/<kind>/README.md`. |
| Should-be-true | Standalone-OSS users (no Synchestra installed) get a complete enough lifecycle experience from `specscore` alone that they do not fall back to hand-editing `**Status:**` lines as a workaround. | Dogfood on the SpecScore meta-spec repo itself for one development cycle. Acceptance: zero `**Status:**` hand-edits in the git log after the verbs ship. |
| Might-be-true | The same `pkg/lifecycle/` abstraction will later serve additional doc kinds (e.g., `proposal`, `entity`, `property` from the in-flight `entity-and-property-definitions` Idea) without refactor. | Design the package interface (`Transition(kind, from, verb) (to, error)`) parameterized on doc kind, not hardcoded to `idea`/`feature`. Verify at design-review time. |
| Might-be-true | Exit code `4` (`InvalidTransition`) does not collide with any existing CLI exit code. | Audit the shared exit-code contract in `spec/features/cli/README.md#shared-exit-code-contract` before locking the number; pick the next free slot if `4` is taken. |


## SpecScore Integration

- **New Features this would create:**
  - `spec/features/cli/idea/approve/` — `idea approve <slug>` (`Draft → Approved`).
  - `spec/features/cli/idea/archive/` — `idea archive <slug>` (any non-terminal → `Archived`).
  - `spec/features/cli/feature/review/` — `feature review <id>` (`Draft → Under Review`).
  - `spec/features/cli/feature/approve/` — `feature approve <id>` (`Draft` or `Under Review → Approved`).
  - `spec/features/cli/feature/implement/` — `feature implement <id>` (`Approved → Implementing`).
  - `spec/features/cli/feature/stabilize/` — `feature stabilize <id>` (`Implementing → Stable`).
  - `spec/features/cli/feature/deprecate/` — `feature deprecate <id>` (`Stable → Deprecated`).
- **Existing Features affected:**
  - [`spec/features/cli/idea/README.md`](../features/cli/idea/README.md) — Contents table expanded with the new sub-features; the *"MVP surface is `idea new`"* line revised to reflect lifecycle coverage; `REQ: ideas-only` reaffirmed (lifecycle verbs MUST NOT mutate `spec/features/`).
  - [`spec/features/cli/feature/README.md`](../features/cli/feature/README.md) — Contents table expanded with the new sub-features.
  - [`spec/features/cli/task/README.md`](../features/cli/task/README.md) — the standing Outstanding Question *"When should lifecycle commands (`task claim`, `task release`, `task status`) land?"* is **closed** with a reference to this Idea's *Architectural positioning* paragraph: Task lifecycle is Synchestra's domain by design; whether `specscore` mirrors a thin task-status primitive is its own future Idea.
  - [`spec/features/cli/README.md`](../features/cli/README.md) — shared exit-code contract gains exit code `4` (`InvalidTransition`).
  - `pkg/lifecycle/` (new internal package) — `Transition(kind, from, verb) (to, error)` plus a per-kind transition table; consumed by `internal/cli/idea.go` and `internal/cli/feature.go`.
  - `pkg/lint/` — re-invoked from each transition command after the file mutation; no API change.
  - `internal/cli/idea.go`, `internal/cli/feature.go` — new cobra subcommand wiring.
  - [`ai-plugin-specscore`](https://github.com/synchestra-io/ai-plugin-specscore) plugin — **follow-on, not in this Idea.** The downstream skill updates (one `references/<verb>.md` per new verb, plus SKILL.md table updates) MUST include a pre-flight detection rule: *if both `specscore` and a corresponding `synchestra <kind> <verb>` command are installed, the skill SHOULD prefer the Synchestra command for that doc kind.* For Idea and Feature today, no Synchestra equivalent exists and `specscore` is always the canonical path; for any future doc kind where Synchestra grows a sibling verb, the rule auto-routes richer-context users to Synchestra without breaking standalone-OSS users.
- **Dependencies:** None blocking. Builds on existing `spec lint --fix` infrastructure (specifically the `idea-index-row-sync` and equivalent feature-index sync rules) and the existing exit-code contract.

## Outstanding Questions

- **Full state-machine matrix.** The Recommended Direction sketches the obvious transitions but does not exhaustively enumerate every legal `(from, verb, to)` triple. Lock down the matrix at spec time before implementation. Open sub-questions: does `feature approve` accept both `Draft → Approved` and `Under Review → Approved`, or strictly require `review` first? Is `Archived` a status value on the Idea front-matter or only a directory location under `spec/ideas/archived/`?
- **Where does the legal-transition matrix live in the spec tree?** Inline inside `spec/features/cli/idea/README.md` and `spec/features/cli/feature/README.md`, or in a shared `spec/features/lifecycle/` Meta feature? The shared location avoids duplication but introduces a new spec node.
- **Plugin-skill Synchestra-detection rule — exact mechanism.** Is `command -v synchestra` sufficient, or does the skill probe a specific subcommand (e.g., `synchestra <kind> approve --help` returns 0) for kind-by-kind detection? Latter is more precise but slower.
- **Exit code `4` for `InvalidTransition` — does it collide?** Audit the existing shared exit-code contract before locking the number. Use the next free slot if 4 is taken.
- **Should `idea reopen` (`Approved → Draft`) ship in MVP?** Rare but the entity-and-property-definitions Idea (in-flight) is the kind of artifact where a substantial post-approval rework might justify a status rollback. Lean: defer; observe demand.
- **Should `--reason` (or `--message`) become a flag on transitions in MVP?** Audit-trail value is real even without event emission. The git commit captures the *what*; `--reason` would capture the *why* in a machine-readable way (front-matter or commit body). Lean: defer.
- **Should `feature deprecate` require the deprecation reason and a successor reference in MVP?** Deprecation without a successor link is a footgun. Either bake it in via mandatory flags or document the convention and enforce via lint.

---
*This document follows the https://specscore.md/idea-specification*
