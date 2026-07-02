---
name: joomail-orchestrator
description: "JooMail 프로젝트의 다단계 구현, 백엔드 하드닝, 프론트엔드 API 연동, QA 검증, 리뷰, 다음 작업 선정, 다시 실행, 재실행, 업데이트, 수정, 보완, 부분 재실행, 이전 결과 기반 개선 요청을 조율하는 프로젝트 전용 오케스트레이터. JooMail에서 범위가 둘 이상의 파일/역할/검증 단계를 넘으면 이 스킬을 사용한다."
---

# JooMail Orchestrator

JooMail 작업을 `AGENTS.md`의 프로젝트 규칙과 역할 지침에 맞춰 조율한다. 현재 핵심 목표는 live IMAP/SMTP 기반 백엔드 foundation과 이미 존재하는 React UI의 안전한 통합이다.

## Execution Mode

- Default: single orchestrator for narrow sequential code changes.
- Parallel workers: use only for bounded read-heavy audits or clearly separated backend, frontend, and QA work.
- Role source: `AGENTS.md` Harness: JooMail Development.
- Execution policy: `.codex/config.toml`.
- Intermediate artifacts: `_workspace/` for multi-agent or multi-session work.

## Required Context

Before implementation, read:
- `AGENTS.md`
- `docs/webmail-ui-plan.md`
- `docs/development-checklist.md`

When visual QA is involved, also read:
- `docs/qa-ui-states.md`

Do not deploy, tag, alter CI/deployment, touch secrets, add persistence, or modify Dovecot/Postfix configuration unless the user explicitly asks in the current conversation.

## Agents

| Role | Runtime | Responsibility | Output |
|---|---|---|---|
| joomail_orchestrator | default | Scope, route, integrate, report | plan and final result |
| joomail_backend_worker | worker | Go HTTP/IMAP/SMTP, MIME/session/API tests | minimal patch and Go test evidence |
| joomail_frontend_worker | worker | React/TypeScript API integration and UI states | minimal patch and typecheck/build evidence |
| joomail_qa_verifier | worker | Boundary coherence, missing tests, QA route and parser coverage | findings or no-finding report |

## Workflow

### Phase 0: Context Check
1. Check `git status --short --branch`.
2. Identify unrelated user changes and keep them out of the task unless explicitly included.
3. If `_workspace/` contains relevant prior artifacts and the user asks for a rerun or update, reuse only the relevant artifact. Otherwise treat the request as a fresh run.
4. Confirm whether the request is backend, frontend, QA, review, docs, or mixed.
5. Check whether the request closes, defers, or creates an item in `docs/development-checklist.md`.

### Phase 1: Scope And Verification Target
1. Map the request to `docs/webmail-ui-plan.md` phase and non-goals.
2. Map the request to a checked or unchecked item in `docs/development-checklist.md`, or note that no tracked item applies.
3. Define a concrete verification command:
   - Backend: `go test ./...`
   - Frontend: `npm run typecheck`, or build when UI/runtime behavior changed materially.
   - Mixed: both commands.
4. Stop and ask before adding dependencies, changing public JSON response field names, replacing the MIME parser wholesale, deploying, or touching secrets/config outside repo scope.

### Phase 2: Execution Routing
- Use `joomail_backend_worker` for `cmd/joomaild`, `internal/httpapi`, IMAP/SMTP, MIME parsing, sessions, attachment download, search, or API tests.
- Use `joomail_frontend_worker` for `src/` UI integration, state views, QA routes, and parsed response rendering.
- Use `joomail_qa_verifier` after meaningful backend/frontend work or when the request is review/audit/QA.
- Keep small single-surface tasks in the main thread instead of spawning workers.

Parallelize only when:
- Workers can complete without each other's immediate output.
- Write ownership is separated, or the task is read-only.
- The result needs integration rather than shared live editing.

### Phase 3: Implementation Rules
1. Backend code must keep product behavior on live IMAP/SMTP data.
2. Frontend code must never parse raw MIME; it consumes backend-parsed fields only.
3. Local fixtures are acceptable for Go parser tests.
4. Use existing project patterns before introducing abstractions.
5. Keep edits surgical. Do not clean unrelated files.
6. Update `docs/development-checklist.md` in the same change when code closes a tracked gap, intentionally defers planned behavior, or reveals a new source-vs-plan mismatch.

### Phase 4: QA And Boundary Verification
Run boundary checks appropriate to the change:
- Go response shapes vs `src/types.ts` and frontend consumers.
- Message state actions vs UI optimistic updates.
- Parser fixture coverage vs `docs/webmail-ui-plan.md` MIME requirements.
- QA query routes vs `docs/qa-ui-states.md`.
- Stale or missing entries in `docs/development-checklist.md`.

Report issues as severity-ordered findings with file paths and concrete evidence. If none, state residual risk and unverified areas.

### Phase 5: Final Verification
Before claiming completion:
1. Run the full verification command selected in Phase 1.
2. Read the exit code and output.
3. Report exact pass/fail state and any warnings.
4. Summarize changed files and note unrelated dirty files that were left untouched.
5. State which checklist items were updated, or state that no checklist item changed.

## Data Flow

```text
User request
  -> context and scope check
  -> optional backend/frontend/QA workers
  -> integrated patch or report
  -> verification evidence
  -> final summary
```

Use `_workspace/{phase}_{role}_{artifact}.md` only for multi-agent tasks or when preserving intermediate audit results is useful.

## Error Handling

| Situation | Strategy |
|---|---|
| Missing docs or unclear scope | Ask before editing |
| Existing unrelated dirty files | Ignore or explicitly exclude from staging |
| Test/build failure | Apply systematic debugging before fixing |
| Dependency appears necessary | Stop and ask |
| Public API contract change appears necessary | Stop and ask |
| Worker result conflicts | Prefer documented product scope and cite both sources |
| Checklist and source disagree | Update the checklist or explain why the source is intentionally deferred |

## Trigger Examples

Use this skill for:
- "다음 백엔드 단계 진행해줘"
- "검색 백엔드 하드닝해줘"
- "MIME 파서 다시 점검해줘"
- "프론트 API 연동 검증해줘"
- "QA 상태 스크린샷 체크해줘"
- "이전 작업 보완/재실행해줘"

Do not use this skill for:
- one-line factual questions
- simple `git status`/`date` style commands
- changes outside JooMail
