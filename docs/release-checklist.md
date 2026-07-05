# JooMail Release Checklist

Use this checklist only after the user explicitly asks for release or deploy work.
Do not push tags, trigger deploy workflows, or change production state without
explicit approval in the current conversation.

## Pre-Release

- Confirm `docs/development-checklist.md` is synchronized with source changes.
- Confirm `docs/future-work-100.md` still has exactly 100 numbered items:
  ```sh
  rg '^### [0-9]{3}\.' docs/future-work-100.md | wc -l
  ```
- Run backend verification when backend files changed:
  ```sh
  go test ./...
  ```
- Run frontend verification when frontend files changed:
  ```sh
  npm run typecheck
  ```
- Run docs whitespace verification:
  ```sh
  git diff --check
  ```
- Review `docs/qa-ui-states.md` and record any manual visual QA or live smoke
  blockers before release sign-off.
- If visual QA is in scope, run:
  ```sh
  npm run qa:visual
  ```

## Release

- Bump version metadata only when the release scope requires it.
- Use an approved release tag such as `joomail-v0.1.13`.
- Push the release tag only after explicit approval.
- Watch the GitHub Actions deploy run only after an approved deploy trigger.

## Post-Release

- Record deploy run URL or release identifier in `docs/qa-ui-states.md`.
- Record live smoke status without credentials or message contents.
- If smoke fails, capture the failing check and stop for an approved remediation
  plan. Use `docs/rollback.md` only after an explicit production rollback
  approval.

## Changelog Decision

JooMail does not maintain a separate changelog file in the current phase.
Release notes should be derived from the git tag, PR/commit summary, and
`docs/development-checklist.md` evidence unless the project explicitly adopts a
maintained changelog later.
