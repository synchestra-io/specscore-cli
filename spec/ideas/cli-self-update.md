# Idea: CLI Self-Update

**Status:** Draft
**Date:** 2026-06-01
**Owner:** alex
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let specscore-cli check for updates and update itself when appropriate, while detecting package-manager installs and redirecting those users to the correct upgrade path instead of replacing managed binaries?

## Context

SpecScore CLI now has enough breadth that users will expect a first-class update path. There are established Go libraries for release-aware self-update, but blindly replacing the executable is only correct for manual installs. Package-managed installs such as Homebrew, Scoop, Nix, apt, or similar should usually be detected and routed to the package manager update command instead. The install-method detection boundary is therefore part of the product, not an implementation detail.

## Recommended Direction

Add a specscore update surface that first detects the install method, then chooses one of two paths. For unmanaged or manual installs, the CLI may offer in-place binary replacement using a release-aware updater library. For package-managed installs, the CLI should not overwrite the executable; it should report the detected manager and print the exact upgrade command the user should run.

Treat install-method detection as a first-class contract. The command should prefer explicit signals such as known installation prefixes, package-manager metadata, executable path heuristics, and release-channel metadata over guesswork. When detection is ambiguous, default to the safe path: do not self-replace, surface the ambiguity, and require the user to update manually.

Keep the MVP focused: version check, install-method detection, safe decisioning, and user-visible guidance. Rollback, channel pinning, signed update verification, and background auto-update are follow-on scope unless the chosen library and release pipeline make them nearly free.

## Alternatives Considered

<!-- 2–3 directions that lost, and why each lost. -->

## MVP Scope

A lint-clean Idea for a specscore update capability that can detect common install methods, report whether self-update is allowed, print package-manager upgrade commands for managed installs, and perform direct self-update only for manual installs. The MVP also decides whether install-method detection belongs in a shared package for reuse by future commands such as version or telemetry.

## Not Doing (and Why)

- Background auto-update daemon — updates run only when the user invokes the command
- Overwriting package-managed binaries — managed installs should be redirected to the package manager
- Multiple release channels at launch — stable-only until release management needs more
- Silent self-replacement on ambiguous detection — ambiguity falls back to manual guidance

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | placeholder dealbreaker assumption | describe how to validate |
| Should-be-true | … | … |
| Might-be-true | … | … |


## SpecScore Integration

- **New Features this would create:** TBD at design time
- **Existing Features affected:** none
- **Dependencies:** none

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/idea-specification*
