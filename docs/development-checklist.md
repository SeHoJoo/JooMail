# JooMail Development Checklist

This checklist tracks the gap between `docs/webmail-ui-plan.md` and the current source tree.

Last audited: 2026-07-04
Audit basis: `AGENTS.md`, `README.md`, `docs/webmail-ui-plan.md`, `docs/qa-ui-states.md`, `internal/httpapi/*`, `src/*`

## Maintenance Rules

- Update this file whenever a task implements, removes, or materially changes planned behavior.
- Before starting a non-trivial task, check whether it closes or changes any unchecked item here.
- When completing an item, change `[ ]` to `[x]`, add the implementation evidence, and note the verification command.
- If source review reveals a new gap between docs and implementation, add it here in the same change.
- Do not mark an item complete based only on intent. Use code evidence and a passing verification command.

## Current Implemented Baseline

- [x] Live IMAP login-backed session flow exists.
  Evidence: `POST /api/login`, `GET /api/accounts`, HMAC session cookie, encrypted credential store.
  Verification: existing `internal/httpapi/server_test.go` auth/session coverage.
- [x] Live mailbox and message APIs exist.
  Evidence: accounts, mailbox tree, message summaries, message detail, attachment download, flag, seen, move routes.
  Verification: existing `internal/httpapi/server_test.go` route coverage.
- [x] MIME parsing is backend-owned and covered by parser fixtures.
  Evidence: multipart alternative/mixed/related, transfer encoding, charset, sanitize, remote image, attachment tests.
  Verification: `go test ./...`.
- [x] MIME parser edge fixtures cover deeper nesting and attachment filename edge cases.
  Evidence: parser tests cover nested mixed inside alternative, related inside alternative, duplicate and missing CIDs, malformed attachment filename fallback, RFC 2231 filenames, long Unicode snippets, and HTML-only messages.
  Verification: `TestParseRawMessageNestedMixedInsideAlternative`, `TestParseRawMessageRelatedInsideAlternativeMapsCIDImages`, `TestParseRawMessageDuplicateContentIDUsesLastInlineImage`, `TestParseRawMessageMissingRelatedCIDImageIsSanitized`, `TestParseRawMessageMalformedAttachmentFilenameUsesFallback`, `TestParseRawMessageRFC2231EncodedFilename`, `TestParseRawMessageLargeUnicodeSnippetTruncatesByRune`, `TestParseRawMessageHTMLOnlyMessage`; `go test ./internal/httpapi`.
- [x] HTML sanitizer fixtures cover CSS remote URLs, SVG data images, raster data images, and forms.
  Evidence: sanitizer tests verify CSS/style remote URLs are stripped, SVG data images are blocked, allowed raster data images remain, and form/input/button elements are removed.
  Verification: `TestSanitizeMailHTMLRemovesCSSRemoteImageURLs`, `TestSanitizeMailHTMLBlocksSVGDataImages`, `TestSanitizeMailHTMLAllowsRasterDataImages`, `TestSanitizeMailHTMLRemovesForms`; `go test ./internal/httpapi`.
- [x] Attachment download policy and header-injection behavior are pinned.
  Evidence: attachment downloads use parsed MIME content type with `application/octet-stream` fallback when absent, and formatted download filenames cannot inject response headers.
  Verification: `TestMessageAttachmentRouteDownloadsDecodedAttachment`, `TestExtractAttachmentPayloadDefaultsMissingContentType`, `TestMessageAttachmentDownloadFilenameCannotInjectHeaders`; `go test ./internal/httpapi`.
- [x] SMTP send with attachments and Sent append exists.
  Evidence: `POST /api/send` reports `sentCopyStored:true` after SMTP delivery plus Sent APPEND, and reports `sentCopyStored:false` with HTTP 200 when delivery succeeds but the Sent copy cannot be stored.
  Verification: `TestSendUsesStoredCredentialForSMTP`, `TestSendAppendFailureReturnsSuccessWithMissingSentCopy`; `go test ./...`.
- [x] Send route rejects missing or malformed recipients before SMTP.
  Evidence: `handleSend` trims To/Cc/Bcc recipients, validates them with the Go standard library, and returns `400` for missing or malformed addresses before opening SMTP; `README.md` documents the policy.
  Verification: `TestSendRejectsInvalidRecipientsBeforeSMTP`; `go test ./internal/httpapi`.
- [x] Send route failure surfaces are pinned and generic.
  Evidence: SMTP auth failure, RCPT rejection, DATA failure, and DATA close failure return `502 failed to send message` without leaking upstream server text. After DATA final acceptance, QUIT is best-effort and Sent APPEND still runs; a later Sent APPEND failure is a successful delivery response with `sentCopyStored:false` so retry cannot duplicate delivery.
  Verification: `TestSendSMTPFailuresReturnGenericBadGateway`, `TestSendQuitFailureAfterDataAcceptanceStillAppendsSentCopy`, `TestSendAppendFailureReturnsSuccessWithMissingSentCopy`; `go test ./internal/httpapi`.
- [x] Bcc privacy behavior is pinned.
  Evidence: route-level SMTP test captures To/Cc/Bcc RCPT commands and confirms generated DATA contains no Bcc header.
  Verification: `TestSendBccRecipientsDoNotLeakInMessageHeaders`; `go test ./internal/httpapi`.
- [x] Send request body size is explicitly capped.
  Evidence: `handleSend` wraps request bodies with `http.MaxBytesReader` at 32 MiB before JSON or multipart parsing; oversized multipart requests return `400 invalid send request`; `README.md` documents the cap.
  Verification: `TestSendRejectsOversizedMultipartRequest`; `go test ./internal/httpapi`.
- [x] React UI consumes backend API for product flow after login.
  Evidence: `src/App.tsx` loads `/api/accounts`, message summaries, details, send, move, seen, flagged.
  Verification: `npm run typecheck`.
- [x] Sanitized HTML mail bodies render without app CSS interference.
  Evidence: desktop `ReadingPane` and mobile reading view render backend-provided `htmlBody` through a sandboxed iframe `srcDoc` instead of injecting mail HTML directly into the app DOM.
  Verification: `src/components/mailRendering.test.tsx`; `npm test`; `npm run typecheck`.
- [x] Dev-only QA routes exist for visual states.
  Evidence: `/?qa=loading`, `/?qa=error`, `/?qa=empty`, `/?qa=search`, `/?qa=search-account`, `/?qa=search-empty`, `/?qa=multiselect`, `/?qa=compose`, `/?qa=remote-images-shown`, `/?qa=quoted-expanded`, `/?qa=long-overflow`, `/?qa=many-attachments`, `/?qa=empty-custom-folder`, `/?qa=nested-tree`, `/?qa=mobile-reading-attachments`, `/?qa=compose-cc-bcc`.
  Verification: `docs/qa-ui-states.md` route list; `src/App.tsx` QA state handling; `npm run typecheck`.

## Backend Gaps

- [x] Replace list-then-filter search with IMAP server-side search.
  Evidence: `messageSummaries` now builds `UID SEARCH NOT DELETED` or `UID SEARCH NOT DELETED TEXT ...` criteria before fetching, and no longer filters `q` after fetching summaries.
  Verification: `TestMessageSummariesUseServerSideSearchBeforeLimit`; `go test ./...`.

- [x] Add explicit search scope handling for current mailbox vs current account.
  Evidence: `GET /api/accounts/{accountID}/mailboxes/{mailboxID}/messages?q=...&scope=mailbox|account` supports mailbox scope and current-account selectable mailbox search without cross-account unified search.
  Verification: `TestMessageSummariesAccountScopeSearchesSelectableMailboxes`; `npm run typecheck`.

- [x] Cover selectable mailbox filtering for account-scope search.
  Evidence: fake LIST tree coverage proves account-scope search skips `\Noselect` parent mailboxes while searching selectable children.
  Verification: `TestAccountScopeSearchSkipsNoselectMailboxes`; `go test ./internal/httpapi`.

- [x] Harden IMAP search query quoting and non-ASCII search behavior.
  Evidence: `searchCriteria` trims empty queries to `NOT DELETED`, prefixes text queries with `NOT DELETED`, quotes spaces/quotes/parentheses with `quoteIMAPString`, and uses `CHARSET UTF-8` for non-ASCII queries.
  Verification: `TestSearchCriteriaQuotesSpecialCharactersAndNonASCII`; `go test ./...`.

- [x] Add predictable fallback for IMAP servers rejecting `CHARSET UTF-8` search.
  Evidence: non-ASCII search retries the same `NOT DELETED TEXT` query without the charset prefix when the server rejects charset search; `README.md` documents the behavior.
  Verification: `TestSearchCriteriaWithoutCharsetOnlyStripsUTF8Prefix`, `TestMessageSummariesRetryNonASCIISearchWithoutCharsetWhenRejected`; `go test ./internal/httpapi`.

- [x] Cover IMAP mailbox quoting for names with quotes and backslashes.
  Evidence: command-capture tests verify SELECT, STATUS, SEARCH, capability-selected COPY fallback, and APPEND paths handle quoted/backslash mailbox names safely. MOVE-capable routing is covered separately by move route tests.
  Verification: `TestIMAPMailboxNamesWithQuotesAndBackslashes`; `go test ./internal/httpapi`.

- [x] Cover nested archive/trash move targets.
  Evidence: move route tests advertise MOVE capability and verify encoded nested target mailbox IDs such as `Work/Archive` and `Work/Trash` are decoded and passed to `UID MOVE`.
  Verification: `TestMessageMoveRouteMovesToNestedArchiveAndTrashTargets`; `go test ./internal/httpapi`.

- [x] Document and test IMAP MOVE fallback behavior.
  Evidence: MOVE capability uses `UID MOVE`; otherwise the backend uses `UID COPY` plus `UID STORE +FLAGS.SILENT (\Deleted)`, attempts only `UID EXPUNGE <uid>` when UIDPLUS exists, and never issues full `EXPUNGE`. A UID EXPUNGE failure after successful COPY/STORE remains move success to avoid duplicate COPY on retry.
  Verification: `TestMessageMoveRouteMovesMessageToTargetMailbox`, `TestMessageMoveRouteUsesUIDExpungeFallbackWhenUIDPlusSupported`, `TestMessageMoveRouteLeavesDeferredDeletionWithoutUIDPlus`, `TestMessageMoveRouteTreatsUIDExpungeFailureAsSuccess`; `go test ./internal/httpapi`.

- [x] Keep message detail GET read-only and seen changes explicit.
  Evidence: detail fetch uses `BODY.PEEK[]` and preserves the IMAP `\Seen` state; only `PATCH /api/messages/{messageID}/seen` issues the seen STORE mutation.
  Verification: `TestMessageDetailDoesNotMarkUnreadMessageSeen`, existing seen route tests; `go test ./internal/httpapi`.

- [x] Exclude deleted messages from regular mailbox lists and searches.
  Evidence: empty, ASCII text, non-ASCII text, mailbox-scope, and account-scope IMAP searches include `NOT DELETED`, including the non-ASCII charset fallback.
  Verification: `TestSearchCriteriaQuotesSpecialCharactersAndNonASCII`, `TestMessageSummariesUseServerSideSearchBeforeLimit`, `TestMessageSummariesRetryNonASCIISearchWithoutCharsetWhenRejected`, `TestMessageSummariesAccountScopeSearchesSelectableMailboxes`; `go test ./internal/httpapi`.

- [x] Remove production mock mail/account storage.
  Evidence: server construction no longer accepts a `Store`, `MockStore` and its hard-coded data are removed, login returns an empty mailbox list until live `/api/accounts` data loads, and protocol fakes remain test-only.
  Verification: `TestLoginStoresEncryptedCredentialAndSetsRememberedSessionCookie`; `rg 'MockStore|type Store struct'`; `go test ./...`.

- [x] Defer partial bulk move failure handling until a per-message result policy exists.
  Evidence: `bulkMoveMessages` now uses sequential per-message move requests, removes successfully moved messages from local state, leaves failed messages selected, and surfaces a bulk action error without adding a new API response contract.
  Verification: `npm run typecheck`.

- [x] Decide and implement unread count source for accounts and mailboxes.
  Evidence: `handleAccounts` applies live IMAP `STATUS mailbox (UNSEEN)` counts and `accountForSession` sums mailbox unread counts into the account unread total.
  Verification: `TestAccountsPopulateUnreadCountsFromIMAPStatus`; `go test ./...`.

- [x] Harden unread count failure behavior per mailbox.
  Evidence: route-level fake IMAP coverage confirms one mailbox `STATUS` failure leaves that mailbox at zero while successful mailbox unread counts still populate the account response.
  Verification: `TestAccountsSkipFailedUnreadCountsPerMailbox`; `go test ./internal/httpapi`.

- [x] Add stronger session-expiry API behavior tests.
  Evidence: route-level expired signed session token test pins generic `401` behavior.
  Verification: `TestProtectedRoutesRejectExpiredSession`; `go test ./...`.

- [x] Add backend search performance guardrails before broad mailbox search.
  Evidence: server-side IMAP search delegates matching to IMAP, fetches only matching UID sets, caps per-mailbox and final account search summaries at `messageSummaryLimit`.
  Verification: `TestMessageSummariesUseServerSideSearchBeforeLimit`, `TestMessageSummariesAccountScopeSearchesSelectableMailboxes`; `go test ./...`.

- [x] Document account-scope search cost limits.
  Evidence: `README.md` states current-account search runs live IMAP search across selectable mailboxes and caps per-mailbox fetches plus the merged sorted result.
  Verification: docs diff; no code test required.

- [x] Validate `messageSummaryLimit` behavior with large live-like mailboxes.
  Evidence: regular mailbox listings fetch the newest 50 UIDs before parsing, and account-scope search caps each selectable mailbox plus the final merged result at `messageSummaryLimit`.
  Verification: `TestMessageSummariesLimitFetchesNewestUIDs`, `TestAccountScopeSearchCapsPerMailboxAndFinalResults`; `go test ./internal/httpapi`.

- [x] Document the message-list load-more path without changing current response fields.
  Evidence: `README.md` states current lists return the newest 50 live IMAP matches and future load-more should use optional `limit` plus UID/date cursor query parameters while preserving the `messages` JSON field.
  Verification: docs diff; no runtime behavior changed.

- [x] Defer envelope-only IMAP summary fetching until response-field parity is designed.
  Evidence: summaries currently keep full MIME fetches because response fields depend on backend parsing; `docs/imap-summary-strategy.md` records the bodystructure-aware future path before any envelope-only switch.
  Verification: docs review; no runtime behavior changed.

## Frontend Gaps

- [x] Introduce a tracked search scope control once backend scope exists.
  Evidence: desktop `MessageList` and mobile `MobileInbox` expose current-mailbox/current-account controls and `App` sends `scope=mailbox|account` with search queries.
  Verification: `npm run typecheck`.

- [x] Debounce live search while keeping input responsive.
  Evidence: `App` keeps immediate search input separate from the API-backed search value and applies a 300ms debounce before live IMAP queries.
  Verification: `npm run typecheck`; no API contract change.

- [x] Empty search has an explicit reset rule.
  Evidence: clearing search immediately clears the query, resets search scope to current mailbox, and clears checked message selection.
  Verification: `npm run typecheck`; `/?qa=search-empty` remains documented.

- [x] Account-scope search results communicate source mailbox and cap.
  Evidence: desktop and mobile rows show mailbox labels for account-scope results, and result summary copy shows `최신 50건` when the current cap is reached.
  Verification: `npm run typecheck`; visual QA still recommended.

- [x] Search highlighting covers multiple occurrences.
  Evidence: `MessageRow.highlight` marks every case-insensitive occurrence in subject/snippet text instead of only the first match.
  Verification: `npm run typecheck`.

- [x] Decide search scope persistence behavior.
  Evidence: search scope remains persisted in `joomail:mail-state`, while search text intentionally does not persist across sessions.
  Verification: documented in `docs/future-work-100.md`; no runtime behavior changed.

- [x] Cover automated search cancellation behavior.
  Evidence: `src/App.test.tsx` verifies a stale message-list response cannot overwrite a newer debounced search result.
  Verification: `npm test`.

- [x] Replace non-virtualized message rendering with a scalable list strategy or document a deliberate deferral.
  Evidence: desktop `MessageList` and mobile `MobileInbox` render visible rows with `@tanstack/react-virtual` and keep a fixed-row fallback for non-browser test environments.
  Verification: `npm test`; `npm run typecheck`; `npm run qa:visual`.

- [x] Add account/mailbox state restoration policy.
  Evidence: `App` persists active account, per-account mailbox, selected-message route state, and search scope in `localStorage` under `joomail:mail-state`; explicit live mailbox-list routes and account/mailbox navigation intentionally start with no selected message.
  Evidence: desktop and mobile list scroll positions persist per account, mailbox, search scope, and search text key.
  Verification: `src/App.test.tsx`; `npm test`; `npm run typecheck`.

- [x] Add browser route integration for account, mailbox, and selected message.
  Evidence: React Router routes `/mail/:accountId/:mailboxId` and `/mail/:accountId/:mailboxId/:messageId` are defined, and `AppShell` synchronizes route params with selected product state.
  Verification: `npm test`; `npm run typecheck`.

- [x] Keep live mailbox-list entry and stale-selection cleanup unselected.
  Evidence: explicit mailbox-list routes ignore persisted message IDs, list completion never substitutes the first row, and live account/mailbox navigation clears message selection while explicit message routes still open their requested detail.
  Verification: `src/App.test.tsx`; `npm test`; `npm run typecheck`.

- [x] Mark unread messages seen only after detail opens, with optimistic rollback.
  Evidence: the shared detail-success path sends `PATCH /api/messages/{messageID}/seen` with `{seen:true}` after GET success, clears unread immediately, and restores unread without closing detail when PATCH fails; row, keyboard, mobile, and direct-route opens share this path.
  Verification: `src/App.test.tsx`; `npm test`; `npm run typecheck`.

- [x] Surface a sent-copy storage warning without treating delivery as failed.
  Evidence: `AppShell` consumes `sentCopyStored`, closes compose on successful delivery, and shows one dismissible responsive `role="status"` notification when the Sent copy was not stored.
  Verification: `src/App.test.tsx`; `npm test`; `npm run typecheck`.

- [ ] Update selected-message manual QA wording for the mailbox-list entry policy.
  Evidence: `docs/qa-ui-states.md` still says selected messages restore from `joomail:mail-state`, while live mailbox-list routes now intentionally start unselected and only explicit message routes restore detail.
  Required: update the manual checklist during the final documentation synchronization task.

- [x] Complete compose overlay controls from the plan or explicitly defer each missing control.
  Evidence: `ComposePanel` now supports desktop minimize/restore and expand/collapse controls.
  Evidence: Draft save is implemented through `POST /api/drafts`; `ComposePanel` saves then closes on successful Drafts append and keeps retryable error state on failure.
  Verification: `TestSaveDraftAppendsToDraftsMailbox`; `go test ./internal/httpapi`; `npm run typecheck`.

- [x] Compose protects dirty unsent content from accidental close.
  Evidence: `ComposePanel` reports dirty state, `App.closeCompose` confirms before discard, and the compose-open history guard routes browser/mobile back through the same confirmation path.
  Verification: `npm run typecheck`; manual mobile back QA still recommended.

- [x] Compose attachment controls cover remove and total size.
  Evidence: selected attachment chips have per-file remove buttons backed by the same `File[]` sent to `/api/send`, and the attachment area displays aggregate selected file size.
  Verification: `npm run typecheck`; manual compose QA checklist updated.

- [x] Compose send button communicates required fields before submit.
  Evidence: send is disabled until recipients and subject are present, and the footer/title exposes the first missing required field.
  Verification: `npm run typecheck`; manual compose QA checklist updated.

- [x] Compose send failure keeps retry state visible.
  Evidence: failed sends preserve compose fields and selected attachments and relabel the send button as `다시 보내기`.
  Verification: `npm run typecheck`; manual compose QA checklist updated.

- [x] Decide MVP forward attachment policy.
  Evidence: forwarding remains body-only by default; original attachments are not automatically reattached, and users can manually attach files to avoid hidden large sends.
  Verification: documented in `docs/future-work-100.md`; no runtime behavior changed.

- [x] Add compose recipient policy unit tests.
  Evidence: `src/components/ComposePanel.test.ts` verifies reply-all self filtering and body-only forward initialization.
  Verification: `npm test`.

- [x] Decide rich-text compose policy for MVP.
  Evidence: compose remains plaintext in the live-backend phase; rich-text formatting is deferred until a backend-owned sanitized HTML send contract exists.
  Verification: documented in `docs/future-work-100.md`; no runtime behavior changed.

- [x] Replace hard-coded attachment total size in the reading pane.
  Evidence: `ReadingPane` computes aggregate attachment size from backend-provided attachment size strings and omits the aggregate when parsing is not possible.
  Verification: `npm run typecheck`.

- [x] Render text URLs and image attachment previews without frontend MIME parsing.
  Evidence: `mailRendering.renderTextWithLinks` autolinks backend-parsed text bodies, and desktop/mobile reading panes display image attachment thumbnails through the existing backend attachment download route.
  Verification: `npm test`; `npm run typecheck`; `npm run qa:visual`.

- [x] Add real quoted-content handling or mark as deferred.
  Evidence: `ReadingPane` detects parsed plain-text quote starts (`>` and `On ... wrote:`), hides quoted paragraphs by default, and expands only when quoted content exists.
  Verification: `npm run typecheck`.

- [x] Add backend-owned conversation threading metadata.
  Evidence: `parseRawMessage` now normalizes `Message-ID`, `In-Reply-To`, and `References`, derives `threadId`, and includes the metadata in parsed message JSON without frontend MIME parsing.
  Verification: `TestParseRawMessageThreadHeaders`; `go test ./internal/httpapi`; `npm run typecheck`.

- [x] Add ManageSieve-backed rules foundation without labels.
  Evidence: `Config` supports optional `JOOMAIL_MANAGESIEVE_HOST`, `JOOMAIL_MANAGESIEVE_PORT`, and `JOOMAIL_MANAGESIEVE_TLS`; `GET`/`PUT /api/accounts/{accountID}/rules` authenticate with the current session credential, use ManageSieve instead of direct Sieve file edits, and replace only the delimited `BEGIN JOOMAIL RULES` / `END JOOMAIL RULES` block.
  Evidence: the rule model supports sender email/domain contains or equals, subject contains, and safe folder moves including Spam and Trash.
  Deferred: labels/tags are explicitly outside this implementation; destructive discard/block rules need a later explicit product decision.
  Verification: `TestRulesRouteUsesManageSieveCredentialAndWritesManagedScript`, `TestRulesRouteReturnsUnavailableWhenManageSieveDisabled`, `TestReplaceJooMailRulesBlockPreservesUserScriptContent`, `TestBuildJooMailRulesBlockGeneratesFolderClassificationSieve`; `go test ./...`.

- [x] Add rules UI for the ManageSieve-backed rules API.
  Evidence: `SettingsPanel` includes a compact rules editor for the current account, loads/saves through `GET`/`PUT /api/accounts/{accountID}/rules`, and only exposes supported sender/subject folder-move actions.
  Evidence: the UI shows ManageSieve-unavailable and unmanaged-script conflict states without adding labels, discard/block, persistence, or frontend MIME parsing.
  Verification: `npm run typecheck`; `go test ./...`.

- [ ] Design scheduled send and undo send now that they are in phase scope.
  Evidence: `docs/webmail-ui-plan.md` and `docs/future-work-100.md` no longer keep scheduled send/undo send as non-goals.
  Required decision: choose browser-local delayed send vs backend queued send for undo, and define scheduled-send persistence/retry semantics before implementation.

## QA And Documentation Gaps

- [x] Run and record visual QA screenshots for all documented QA routes.
  Evidence: `tests/visual/qa-routes.spec.ts` captures all documented QA routes at desktop and mobile viewport sizes into ignored `docs/qa-screenshots/YYYY-MM-DD/`, and the QA results log records the local capture.
  Verification: `npm run qa:visual`.

- [x] Add expanded dev-only QA routes for newer UI states.
  Evidence: query routes now cover account-scope search, displayed remote images, expanded quoted content, long sender/subject overflow, many attachments, empty custom folder, nested mailbox tree, mobile reading with attachments, and mobile compose with Cc/Bcc open.
  Verification: `npm run typecheck`; `docs/qa-ui-states.md` route table.

- [x] Document QA verification commands, screenshot policy, smoke checks, and manual accessibility passes.
  Evidence: `docs/qa-ui-states.md` records the 100-item count command, screenshot storage policy, live IMAP/SMTP/session-expiry smoke checklists, keyboard/icon/contrast review items, 320px mobile review, wide-desktop review, and production smoke recording rules.
  Deferred: deployed `joomail-v0.1.9` visual QA and live/production smoke execution remain deferred because this batch did not open deployed URLs, use credentials, or run deployment workflows.
  Verification: `git diff --check`; `rg '^### [0-9]{3}\.' docs/future-work-100.md | wc -l`.

- [x] Account switcher supports keyboard account selection.
  Evidence: `AccountSwitcher` opens from the trigger, focuses the selected account, supports ArrowUp/ArrowDown/Home/End within the account list, and selects with Enter/Space.
  Verification: `npm run typecheck`; manual accessibility QA remains recommended.

- [x] Desktop sidebar supports collapsed mode.
  Evidence: `Sidebar` has an expanded layout and a collapsed 64px icon rail preserving account context, compose access, mailbox shortcuts, unread counts, and an expand control.
  Verification: `npm run typecheck`; visual QA still needed.

- [x] Tablet layout uses a coherent sidebar/list/reading arrangement.
  Evidence: the `md` to `<xl` desktop shell uses the 64px sidebar rail while the full sidebar returns at `xl`, and `Toolbar` search/action spacing flexes for tablet widths.
  Verification: `npm run typecheck`; tablet visual QA still recommended.

- [x] Desktop message rows expose hover/focus actions.
  Evidence: `MessageRow` shows archive, trash, and mark-unread action buttons on hover/focus and wires them through existing `App` message actions.
  Verification: `npm run typecheck`; hover visual QA still recommended.

- [x] Desktop list selection supports range and modifier semantics.
  Evidence: `MessageRow` forwards Shift and Cmd/Ctrl click modifiers, while `App` maintains a last-checked anchor for range selection and toggles individual rows on modifier click.
  Verification: `npm run typecheck`; manual QA checklist updated.

- [x] Add a recurring source-vs-docs audit entry after each completed development batch.
  Evidence: this checklist was updated alongside backend/frontend changes with evidence, deferrals, and verification notes.
  Verification: `git diff -- docs/development-checklist.md`.

- [x] Keep README API list synchronized with backend routes.
  Evidence: README endpoint list still matches `Server.routes()` and documents the existing message-list `q` and `scope=mailbox|account` query options.
  Verification: compared `internal/httpapi/server.go` routes against README endpoint list.

- [x] Document release hygiene without triggering deployment.
  Evidence: README uses the current `joomail-v0.1.10` tag example, documents env var semantics, TLS modes, credential lifecycle, and health response; `docs/release-checklist.md` records pre-release checks, approval-before-tag/deploy guardrails, smoke recording, and the current no-changelog decision.
  Evidence: `docs/rollback.md` documents rollback steps, and `.github/workflows/deploy.yml` uses current major action tags verified from upstream.
  Verification: docs diff; `git ls-remote --tags` for selected action tags; no deployment workflow, tag, push, or rollback action run.

## Explicit Non-Goals To Keep Out

These are planned exclusions, not missing work:

- Do not implement unified inbox unless the product plan changes.
- Do not implement labels/tags, destructive discard/block rules, snooze, contacts, calendar integration, account-wide unified search, list sorting options, comfort-density mode, mobile swipe gestures, advanced rich-text formatting, multiple compose windows, or dark-mode toggle UI without a separate product decision.
