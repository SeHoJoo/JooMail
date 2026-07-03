# JooMail Future Work 100

Last audited: 2026-07-03  
Audit basis: `AGENTS.md`, `.agents/skills/joomail-orchestrator/SKILL.md`, `README.md`, `package.json`, `docs/webmail-ui-plan.md`, `docs/development-checklist.md`, `docs/qa-ui-states.md`, `.github/workflows/deploy.yml`, `internal/httpapi/server.go`, `internal/httpapi/imap.go`, `internal/httpapi/smtp.go`, `internal/httpapi/login.go`, `internal/httpapi/session.go`, `internal/httpapi/server_test.go`, `src/App.tsx`, `src/types.ts`, `src/components/MessageList.tsx`, `src/components/MobileInbox.tsx`, `src/components/ReadingPane.tsx`, `src/components/ComposePanel.tsx`, `src/components/Toolbar.tsx`, `src/components/Sidebar.tsx`

## Rules

- Keep this file as a planning backlog, not a completion checklist.
- Add or remove items only after comparing source, docs, and current phase scope.
- Mark any item that needs a dependency, persistence, background sync, CI/deploy, database, or mail-server configuration as `Needs approval`.
- Do not turn explicit non-goals from `docs/webmail-ui-plan.md` into implementation tasks without a separate product decision.
- When an item is implemented, move evidence into `docs/development-checklist.md` or replace the item here with a newly discovered future task.

## Summary

The deployed `joomail-v0.1.9` baseline has live IMAP/SMTP product flow, backend-owned MIME parsing, search scope, unread counts, compose send, and the source-vs-plan checklist closed. The highest-value remaining work is visual QA capture, scalable list rendering, scroll-state restoration, Drafts behavior, deeper IMAP/SMTP hardening, mobile parity, and release/documentation hygiene. Several items below require approval because they touch dependencies, persistence, CI/deploy, background sync, or server configuration.

## 100-Item Backlog

Section: Backend / IMAP / SMTP

### 001. Validate `messageSummaryLimit` behavior with large live-like mailboxes
Status: Completed
Evidence: `internal/httpapi/imap.go` caps message summaries with `messageSummaryLimit = 50`.
Completed evidence: `TestMessageSummariesLimitFetchesNewestUIDs` documents that regular mailbox listing fetches the newest 50 UIDs before summary parsing; `TestAccountScopeSearchCapsPerMailboxAndFinalResults` documents per-mailbox and final account-scope caps.
Verification: `go test ./internal/httpapi`.

### 002. Add a documented pagination or load-more decision
Status: Completed
Evidence: The API currently returns a capped list with no cursor, page token, or offset.
Completed evidence: `README.md` documents that current message lists return the newest 50 live IMAP matches and that future load-more support should add optional `limit` plus UID/date cursor query parameters while keeping the existing `messages` JSON field stable.
Verification: Docs diff; no runtime behavior changed.

### 003. Add IMAP `UID FETCH` envelope-only exploration
Status: Deferred
Evidence: `fetchMessages` fetches `BODY.PEEK[]` for summaries, which retrieves full message bodies.
Deferred rationale: Current summary fields still come from backend MIME parsing, including snippets, attachment presence, decoded headers, and body-derived fallback behavior. Envelope/header-only fetching should wait for a bodystructure-aware summary parser design so existing response fields do not silently degrade.
Verification: Source review of `fetchMessages` and `parseRawMessage`; no runtime behavior changed.

### 004. Harden unread count failures per mailbox
Status: Completed
Evidence: `withUnreadCounts` silently skips STATUS failures and keeps the count at zero.
Completed evidence: `TestAccountsSkipFailedUnreadCountsPerMailbox` pins graceful degradation when one mailbox `STATUS` command fails while other mailbox counts still populate the account response.
Verification: `go test ./internal/httpapi`.

### 005. Add selectable mailbox filtering tests for account-scope search
Status: Completed
Evidence: `accountMessageSummaries` uses `listMailboxNames`, which skips `\Noselect` entries.
Completed evidence: `TestAccountScopeSearchSkipsNoselectMailboxes` covers a fake LIST tree with a `\Noselect` parent and selectable child, proving account-scope search skips the parent and searches the child.
Verification: `go test ./internal/httpapi`.

### 006. Document account-scope search cost limits
Status: Completed
Evidence: Account-scope search loops over every selectable mailbox in the current account.
Completed evidence: `README.md` explains that account-scope search is live IMAP across selectable mailboxes, with per-mailbox and merged-result caps.
Verification: Docs diff; no code test required.

### 007. Add IMAP charset fallback test for servers rejecting `CHARSET UTF-8`
Status: Completed
Evidence: `searchCriteria` emits `CHARSET UTF-8` for non-ASCII queries.
Completed evidence: `searchMailbox` retries non-ASCII `TEXT` search without the `CHARSET UTF-8` prefix when the server rejects charset search; `README.md` documents the fallback.
Verification: `TestMessageSummariesRetryNonASCIISearchWithoutCharsetWhenRejected`; `go test ./internal/httpapi`.

### 008. Add tests for IMAP mailbox names with quotes and backslashes
Status: Completed
Evidence: mailbox names are quoted with `quoteIMAPString`.
Completed evidence: `TestIMAPMailboxNamesWithQuotesAndBackslashes` uses mailbox names containing quotes and backslashes and verifies SELECT, STATUS, SEARCH, MOVE, COPY fallback, and APPEND command paths receive the decoded mailbox names.
Verification: `go test ./internal/httpapi`.

### 009. Add tests for nested archive/trash move targets
Status: Completed
Evidence: `moveToKind` and backend `moveMessage` use mailbox IDs from the returned tree.
Completed evidence: `TestMessageMoveRouteMovesToNestedArchiveAndTrashTargets` verifies encoded nested `Work/Archive` and `Work/Trash` target mailbox IDs are decoded and passed to IMAP MOVE.
Verification: `go test ./internal/httpapi`; no frontend changes.

### 010. Add IMAP MOVE fallback behavior documentation
Status: Completed
Evidence: `moveMessage` falls back to COPY, STORE `\Deleted`, and EXPUNGE when MOVE is not OK.
Completed evidence: `README.md` documents the MOVE fallback, and `TestMessageMoveRouteFallsBackToCopyStoreExpunge` pins the COPY, STORE `\Deleted`, and EXPUNGE sequence.
Verification: `go test ./internal/httpapi`.

### 011. Add partial bulk move failure handling plan
Status: Deferred
Evidence: `bulkMoveMessages` uses `Promise.all`, so one failure rolls back local UI after some server moves may have succeeded.
Deferred rationale: Current API returns one result per move request and has no bulk per-message result contract. Future work should choose sequential moves with per-message UI recovery or introduce a bulk endpoint with per-message results before changing optimistic rollback behavior.
Verification: Source review of `bulkMoveMessages`; no runtime behavior changed.

### 012. Add SMTP recipient validation tests
Status: Completed
Evidence: `sendMail` trims recipients but accepts most non-empty strings before SMTP RCPT.
Completed evidence: `handleSend` trims To/Cc/Bcc recipients and rejects missing or malformed addresses with `400` before SMTP; `README.md` documents the client-error policy.
Verification: `TestSendRejectsInvalidRecipientsBeforeSMTP`; `go test ./internal/httpapi`.

### 013. Add SMTP failure surface tests
Status: Completed
Evidence: `handleSend` maps send errors to `502 failed to send message`.
Completed evidence: `TestSendSMTPFailuresReturnGenericBadGateway` covers SMTP auth failure, RCPT rejection, DATA command failure, and DATA close failure; `TestSendAppendFailureReturnsGenericBadGatewayAfterSMTP` covers Sent append failure.
Verification: `go test ./internal/httpapi`.

### 014. Decide Sent append failure semantics
Status: Completed
Evidence: `sendMail` sends SMTP successfully, then `appendSentMessage` can still fail.
Completed evidence: Current semantics remain unchanged: Sent append failure returns generic `502 failed to send message` after SMTP delivery rather than adding a new sent-with-warning response field. `README.md` documents the decision.
Verification: `TestSendAppendFailureReturnsGenericBadGatewayAfterSMTP`; `go test ./internal/httpapi`.

### 015. Add request size limits for send attachments
Status: Completed
Evidence: `parseMultipartSendRequest` calls `ParseMultipartForm(32 << 20)`, but HTTP body size is not explicitly limited.
Completed evidence: `handleSend` wraps the request body with `http.MaxBytesReader` using a 32 MiB cap before parsing send requests; `README.md` documents the cap.
Verification: `TestSendRejectsOversizedMultipartRequest`; `go test ./internal/httpapi`.

Section: MIME / Security / Attachments

### 016. Expand MIME fixtures for nested multipart mixed inside alternative
Status: Completed
Evidence: Parser tests cover common multipart structures but more nesting combinations are possible.
Completed evidence: `TestParseRawMessageNestedMixedInsideAlternative` covers a nested `multipart/mixed` branch inside `multipart/alternative`, preserving fallback text, HTML body, and attachment separation.
Verification: `go test ./internal/httpapi`.

### 017. Expand MIME fixtures for related inside alternative
Status: Completed
Evidence: CID mapping exists for `multipart/related`.
Completed evidence: `TestParseRawMessageRelatedInsideAlternativeMapsCIDImages` covers a `multipart/related` HTML part inside `multipart/alternative`, preserving fallback text and mapping the CID image.
Verification: `go test ./internal/httpapi`.

### 018. Add fixture for duplicate Content-ID inline images
Status: Completed
Evidence: CID image mapping is keyed by content ID.
Completed evidence: `TestParseRawMessageDuplicateContentIDUsesLastInlineImage` documents that duplicate CID parts resolve predictably to the last inline image and remain excluded from attachments.
Verification: `go test ./internal/httpapi`.

### 019. Add fixture for missing related CID attachment
Status: Completed
Evidence: Related HTML can reference `cid:` values without matching parts.
Completed evidence: `TestParseRawMessageMissingRelatedCIDImageIsSanitized` verifies unresolved `cid:` image sources are removed by sanitization without marking remote images or adding attachments.
Verification: `go test ./internal/httpapi`.

### 020. Add fixture for malformed Content-Disposition filename
Status: Completed
Evidence: Attachments depend on MIME headers and filenames.
Completed evidence: malformed attachment disposition parsing now keeps clear attachment parts visible with the fallback name `attachment`; `TestParseRawMessageMalformedAttachmentFilenameUsesFallback` covers the behavior.
Verification: `go test ./internal/httpapi`.

### 021. Add fixture for RFC 2231 encoded filenames
Status: Completed
Evidence: Attachment filenames from non-English clients may be encoded across parameters.
Completed evidence: `TestParseRawMessageRFC2231EncodedFilename` verifies RFC 2231 `filename*` parameters decode to visible non-English attachment names.
Verification: `go test ./internal/httpapi`.

### 022. Add fixture for very large text body snippet generation
Status: Completed
Evidence: `parseRawMessage` truncates snippets to 140 runes.
Completed evidence: `TestParseRawMessageLargeUnicodeSnippetTruncatesByRune` verifies long Unicode snippets truncate at 140 runes without splitting a character.
Verification: `go test ./internal/httpapi`.

### 023. Add fixture for HTML-only message with no text fallback
Status: Completed
Evidence: HTML body is rendered from backend-parsed `htmlBody`.
Completed evidence: `TestParseRawMessageHTMLOnlyMessage` verifies HTML-only mail produces sanitized `htmlBody` without requiring frontend text fallback or raw MIME parsing.
Verification: `go test ./internal/httpapi`; no frontend changes.

### 024. Add fixture for text-only URL autolink decision
Status: Deferred
Evidence: `docs/webmail-ui-plan.md` mentions URL autolink for text fallback, but current frontend renders plain paragraphs.
Deferred rationale: URL autolink remains intentionally deferred because changing backend text rendering would alter displayed content and needs a parser/rendering policy decision; frontend must continue rendering backend-parsed plain text without raw MIME parsing.
Verification: source review; no runtime behavior changed.

### 025. Add sanitization fixtures for CSS-based remote image URLs
Status: Completed
Evidence: Remote image blocking currently targets `<img src="http(s)://...">`.
Completed evidence: `TestSanitizeMailHTMLRemovesCSSRemoteImageURLs` verifies style attributes and CSS remote URLs are removed and do not trigger the image-display toggle.
Verification: `go test ./internal/httpapi`.

### 026. Add sanitization fixtures for SVG and data image edge cases
Status: Completed
Evidence: `dataImageSrcPattern` allows specific raster image data URLs.
Completed evidence: `TestSanitizeMailHTMLBlocksSVGDataImages` verifies SVG data images are removed, while `TestSanitizeMailHTMLAllowsRasterDataImages` preserves allowed raster data images.
Verification: `go test ./internal/httpapi`.

### 027. Add sanitizer fixture for form elements
Status: Completed
Evidence: `bluemonday` policy is used for HTML sanitize.
Completed evidence: `TestSanitizeMailHTMLRemovesForms` verifies form, input, button, and form action content are removed while safe paragraph content remains.
Verification: `go test ./internal/httpapi`.

### 028. Add attachment content sniffing decision
Status: Completed
Evidence: Attachment download returns stored `ContentType`; multipart upload uses file header content type.
Completed evidence: Current policy remains header-based for this phase: attachment downloads use the parsed MIME content type and fall back to `application/octet-stream` when absent, without content sniffing. `TestMessageAttachmentRouteDownloadsDecodedAttachment` and `TestExtractAttachmentPayloadDefaultsMissingContentType` cover the behavior.
Verification: `go test ./internal/httpapi`.

### 029. Add attachment filename header injection tests
Status: Completed
Evidence: Download uses `mime.FormatMediaType` for filename.
Completed evidence: `TestMessageAttachmentDownloadFilenameCannotInjectHeaders` verifies a decoded CRLF filename cannot create raw newlines or injected response headers.
Verification: `go test ./internal/httpapi`.

### 030. Add attachment thumbnail strategy
Status: Deferred
Evidence: The plan mentions image attachment thumbnails; current reading pane displays a generic image icon.
Deferred rationale: Thumbnail generation/rendering remains deferred because it needs a UI/backend strategy for image preview URLs, sizing, and privacy behavior; current phase keeps generic attachment chips.
Verification: source review of `ReadingPane`; no runtime behavior changed.

Section: Frontend Mail UI

### 031. Capture visual QA for all documented QA routes
Status: Approval-blocked
Evidence: `docs/qa-ui-states.md` has a Pending/Deferred result row.
Approval-blocked rationale: Browser screenshot capture cannot be run in this workspace without adding a browser automation dependency or using an external browser-agent. The approval matrix does not grant browser automation dependency approval.
Verification: `node` package resolution confirmed `playwright`, `@playwright/test`, and `puppeteer` are not installed; no screenshots captured.

### 032. Add QA route for account-scope search
Status: Completed
Evidence: Existing QA routes include search and search-empty, but scope selector state is newer.
Completed evidence: `/?qa=search-account` selects account-scope search state on desktop and mobile, and `docs/qa-ui-states.md` documents screenshot names.
Verification: `npm run typecheck`.

### 033. Add QA route for remote-image displayed state
Status: Completed
Evidence: QA checklist mentions remote-image toggle but query routes do not force it.
Completed evidence: `/?qa=remote-images-shown` opens the reading state with remote images marked displayed by default, and `docs/qa-ui-states.md` documents the route.
Verification: `npm run typecheck`.

### 034. Add QA route for quoted-content expanded state
Status: Completed
Evidence: ReadingPane collapses quoted content, but QA routes do not force expanded content.
Completed evidence: `/?qa=quoted-expanded` selects a quoted-message fixture and opens quoted content by default for capture.
Verification: `npm run typecheck`.

### 035. Add QA route for long sender and subject overflow
Status: Completed
Evidence: Message rows use absolute positions and truncation.
Completed evidence: `/?qa=long-overflow` selects a dev-only message with long sender, address, subject, and snippet values; docs include the route and checklist item.
Verification: `npm run typecheck`.

### 036. Add QA route for many attachments
Status: Completed
Evidence: ReadingPane wraps attachment chips but has no dedicated route for dense attachment lists.
Completed evidence: `/?qa=many-attachments` selects a dev-only message with many long attachment names; docs include the route and checklist item.
Verification: `npm run typecheck`.

### 037. Add QA route for empty selectable custom folder
Status: Completed
Evidence: Folder tree supports custom folders, and empty state exists.
Completed evidence: `/?qa=empty-custom-folder` selects the custom `Clients` folder, which has no mock messages, to verify non-Inbox empty state.
Verification: `npm run typecheck`.

### 038. Add QA route for nested mailbox tree
Status: Completed
Evidence: Backend and UI support nested folders.
Completed evidence: `/?qa=nested-tree` selects the nested `Clients` folder and opens the mobile folder drawer by default for mobile capture.
Verification: `npm run typecheck`.

### 039. Add QA route for mobile reading pane with attachments
Status: Completed
Evidence: MobileReadingPane renders message body but attachment parity needs visual verification.
Completed evidence: `/?qa=mobile-reading-attachments` opens the mobile reading pane directly on a dev-only message with attachments.
Verification: `npm run typecheck`.

### 040. Add QA route for mobile compose with Cc/Bcc open
Status: Completed
Evidence: ComposePanel supports Cc/Bcc expansion.
Completed evidence: `/?qa=compose-cc-bcc` opens compose with Cc/Bcc fields expanded by default; docs include the route and checklist item.
Verification: `npm run typecheck`.

### 041. Implement true list virtualization
Status: Approval-blocked
Evidence: `MessageList` and `MobileInbox` map every visible message; docs mention `@tanstack/react-virtual`.
Approval-blocked rationale: The likely implementation requires a list virtualization dependency such as `@tanstack/react-virtual`, and the approval matrix does not grant list virtualization dependency approval.
Verification: No dependency added; no runtime behavior changed.

### 042. Restore list scroll position per account and mailbox
Status: Deferred
Evidence: `App` persists account/mailbox/message state, but not scroll offsets.
Deferred rationale: Scroll restoration remains deferred until true virtualization or a stable scroll-container policy is approved, because current non-virtualized list rendering does not define durable per-mailbox scroll anchors.
Verification: No runtime behavior changed.

### 043. Add browser history or route integration decision
Status: Approval-blocked
Evidence: `docs/webmail-ui-plan.md` proposes `/mail/...` routes, but current app is state-driven and package has no router dependency.
Approval-blocked rationale: React Router approval is not granted in the approval matrix, so the MVP remains URL-less state-driven for now.
Verification: No dependency added; no runtime behavior changed.

### 044. Add visible selected-message state restoration test plan
Status: Completed
Evidence: `joomail:mail-state` persists selected message ID in localStorage.
Completed evidence: `docs/qa-ui-states.md` includes a manual checklist item for selected account/mailbox/message restoration through `joomail:mail-state`.
Verification: docs diff; no runtime behavior changed.

### 045. Add account switcher keyboard navigation
Status: Completed
Evidence: Account switching is UI-driven through AccountSwitcher.
Completed evidence: `AccountSwitcher` supports ArrowUp/ArrowDown/Home/End navigation within the account list, focuses the active account when opened, and selects with Enter/Space.
Verification: `npm run typecheck`; manual accessibility QA remains recommended before release.

### 046. Add sidebar collapse behavior
Status: Completed
Evidence: Plan says the sidebar can collapse; current Sidebar is fixed width.
Completed evidence: `Sidebar` now supports an expanded 248px layout and a collapsed 64px icon rail that preserves current account context, compose access, mailbox shortcuts, unread counts, and an expand control.
Verification: `npm run typecheck`; visual QA still needed for desktop capture.

### 047. Add tablet layout pass for 768-1279px
Status: Completed
Evidence: Plan specifies a tablet range; current CSS mainly uses `md` desktop switch.
Completed evidence: The desktop shell now uses a compact 64px sidebar rail from `md` through `<xl`, while the full 248px sidebar returns at `xl`; toolbar spacing also flexes so search and action controls fit tablet widths without forcing the sidebar/list/reading panes to overlap.
Verification: `npm run typecheck`; visual QA at representative tablet widths remains recommended.

### 048. Add message row hover actions
Status: Completed
Evidence: Plan calls for hover archive/delete/read actions; current row actions are mostly list-level and reading-pane actions.
Completed evidence: Desktop `MessageRow` now exposes hover/focus row actions for archive, trash, and mark unread, wired through the existing parsed-message API actions in `App` without changing row dimensions.
Verification: `npm run typecheck`; visual QA still needed for hover captures.

### 049. Add shift-click range selection
Status: Completed
Evidence: Plan includes Shift+click range selection; current selection toggles individual rows.
Completed evidence: `App` tracks the last checked anchor and desktop `MessageRow` forwards Shift-click modifiers so contiguous visible message ranges are selected in `MessageList`.
Verification: `npm run typecheck`; manual QA checklist updated.

### 050. Add Cmd/Ctrl multi-select verification
Status: Completed
Evidence: Plan includes Cmd/Ctrl+click selection semantics.
Completed evidence: Desktop `MessageRow` forwards Cmd/Ctrl-click modifiers to toggle an individual message while keeping the row selected, and `docs/qa-ui-states.md` now records manual verification for modifier selection.
Verification: `npm run typecheck`; manual QA checklist updated.

Section: Compose

### 051. Implement Drafts save API
Status: Approval-blocked
Evidence: Compose shows a deferred Drafts notice and no backend Drafts API exists.
Deferred rationale: Drafts 저장 API 구현 approval was not granted in the request matrix, and current phase excludes persistence-like Drafts behavior unless explicitly approved.
Verification: no runtime behavior changed.

### 052. Add save-to-Drafts then close behavior
Status: Approval-blocked
Evidence: Plan calls for "save to drafts then close"; current button only shows a notice.
Deferred rationale: This depends on the blocked Drafts save API and would otherwise imply unapproved Drafts persistence behavior.
Verification: no runtime behavior changed.

### 053. Add close confirmation for dirty compose
Status: Completed
Evidence: Compose close currently calls `onClose` directly.
Completed evidence: `ComposePanel` reports dirty state to `App`, and `closeCompose` now confirms before closing non-empty unsent compose content from close, delete, Escape, or other close paths.
Verification: `npm run typecheck`; manual compose QA checklist updated.

### 054. Add mobile back behavior for dirty compose
Status: Completed
Evidence: Plan says mobile back should confirm draft handling.
Completed evidence: While compose is open, `App` pushes a lightweight history marker and routes `popstate` through the same dirty-close confirmation, re-adding the marker if the user cancels discard.
Verification: `npm run typecheck`; manual mobile back QA checklist updated.

### 055. Add reply-all recipient tests for self filtering
Status: Deferred
Evidence: `composeInitialState` filters the current account email from recipients.
Deferred rationale: Reply-all filtering exists in `composeInitialState`, but adding a frontend unit-test runner or dependency was not approved in this batch. Keep this as a test gap until frontend test setup is approved.
Verification: `npm run typecheck`; no test tooling added.

### 056. Add forward attachment policy decision
Status: Completed
Evidence: Forwarded body includes text, but existing message attachments are not automatically attached.
Completed evidence: MVP keeps forwarding body-only by default; original attachments are not automatically reattached, and users can attach files manually to avoid hidden large attachment sends.
Verification: documented decision; no runtime behavior changed.

### 057. Add rich text minimum formatting decision
Status: Deferred
Evidence: Plan permits bold/italic/link/list minimal formatting; current compose is plain textarea.
Expected outcome: Decide whether MVP remains plaintext or adds minimal formatting.
Verification: Requires UI design and typecheck if implemented.

### 058. Add compose attachment removal controls
Status: Completed
Evidence: Compose displays selected attachment chips but does not expose per-file removal.
Completed evidence: selected compose attachments now render per-file remove buttons that update the actual `File[]` sent to `/api/send`.
Verification: `npm run typecheck`; manual compose QA checklist updated.

### 059. Add compose attachment total-size indicator
Status: Completed
Evidence: Compose lists per-file sizes only.
Completed evidence: compose attachment rows now show count and aggregate outgoing size computed from the selected `File[]`.
Verification: `npm run typecheck`; manual visual QA checklist updated.

### 060. Add compose send disabled state for missing required fields
Status: Completed
Evidence: Backend rejects missing To/Subject; frontend send button remains active.
Completed evidence: compose send is disabled until at least one recipient and a subject are present, and the footer/title communicates the first missing required field.
Verification: `npm run typecheck`; manual compose QA checklist updated.

### 061. Add Bcc privacy regression test plan
Status: Completed
Evidence: SMTP format omits Bcc header while recipients include Bcc.
Completed evidence: route-level SMTP test captures RCPT commands for To/Cc/Bcc recipients and asserts the generated message data does not include a Bcc header.
Verification: `TestSendBccRecipientsDoNotLeakInMessageHeaders`; `go test ./internal/httpapi`.

### 062. Add send progress and retry behavior
Status: Completed
Evidence: Compose shows `sending` and error message but no retry-specific UX.
Completed evidence: failed sends keep compose fields and selected attachments in place, show the existing error message, and relabel the send button as `다시 보내기` for explicit retry.
Verification: `npm run typecheck`; manual compose QA checklist updated.

Section: Search

### 063. Add search debounce decision
Status: Completed
Evidence: `handleSearch` updates state on every input change, triggering live API calls.
Completed evidence: `App` now separates immediate search input from the debounced live query and applies a 300ms debounce before changing the API-backed `search` value.
Verification: `npm run typecheck`; backend command-count testing remains unnecessary because the API contract did not change.

### 064. Add search cancellation behavior tests
Status: Deferred
Evidence: App effects use a `cancelled` flag for stale responses.
Deferred rationale: the stale-response guard already exists in `App` effects, but adding automated frontend tests requires a frontend test setup/dependency that was not approved in this batch. Keep manual QA coverage until test tooling is approved.
Verification: `npm run typecheck`; no test dependency added.

### 065. Add search empty-query UX rule
Status: Completed
Evidence: Empty search omits query params and returns normal mailbox listing.
Completed evidence: clearing search now clears the visible input and debounced query immediately, resets search scope to current mailbox, and clears checked selection state.
Verification: `npm run typecheck`; search-empty QA route remains in `docs/qa-ui-states.md`.

### 066. Add non-ASCII search live-server compatibility note
Status: Completed
Evidence: Backend sends `CHARSET UTF-8` for non-ASCII search queries.
Completed evidence: `README.md` documents that non-ASCII search uses `CHARSET UTF-8` first and retries without the charset prefix if the IMAP server rejects charset search.
Verification: docs review; no runtime behavior changed.

### 067. Add search result mailbox label for account-scope results
Status: Completed
Evidence: Account-scope results may include messages from multiple mailboxes.
Completed evidence: desktop and mobile account-scope search rows now show a compact mailbox label sourced from each parsed message `mailboxId`.
Verification: `npm run typecheck`; visual QA still recommended.

### 068. Add search highlight for multiple occurrences
Status: Completed
Evidence: `MessageRow.highlight` marks only the first occurrence.
Completed evidence: `MessageRow.highlight` now walks the full subject/snippet string and marks every case-insensitive occurrence.
Verification: `npm run typecheck`.

### 069. Add search scope persistence decision
Status: Completed
Evidence: `joomail:mail-state` stores searchScope, but not search text.
Completed evidence: current behavior is retained intentionally: search scope persists in `joomail:mail-state`, while search text does not persist across sessions.
Verification: documented decision; no runtime behavior changed.

### 070. Add account-scope search result cap communication
Status: Completed
Evidence: Backend caps account-scope results at `messageSummaryLimit`.
Completed evidence: desktop and mobile account-scope search copy now displays `최신 50건` when the visible result count reaches the current live search cap.
Verification: `npm run typecheck`; visual QA still recommended.

Section: QA / Testing

### 071. Add automated count check for `docs/future-work-100.md`
Status: Completed
Evidence: This backlog requires exactly 100 numbered items.
Completed evidence: `docs/qa-ui-states.md` now records the exact `rg '^### [0-9]{3}\.' docs/future-work-100.md | wc -l` verification command.
Verification: command returns 100.

### 072. Add visual QA screenshot storage policy
Status: Completed
Evidence: `docs/qa-ui-states.md` says not to commit screenshots unless requested.
Completed evidence: `docs/qa-ui-states.md` now defines default local screenshot folders, per-pass retention, commit policy, and external artifact recording.
Verification: docs diff only.

### 073. Add QA pass for deployed `joomail-v0.1.9`
Status: Deferred
Evidence: Release was deployed, but visual QA log remains pending.
Deferred rationale: deployed visual QA was not executed from this workspace; `docs/qa-ui-states.md` now records a deferred `joomail-v0.1.9` row with the required blocker.
Verification: docs diff only; no deployed URL was opened.

### 074. Add manual live IMAP smoke checklist
Status: Completed
Evidence: Automated tests use fake IMAP/SMTP; product behavior uses live servers.
Completed evidence: `docs/qa-ui-states.md` now includes a live IMAP smoke checklist covering login, mailbox list, nested mailboxes, message open, backend-parsed detail, attachment download, and logout.
Verification: docs diff only; no live credentials used.

### 075. Add manual SMTP send smoke checklist
Status: Completed
Evidence: Backend has SMTP tests, but live SMTP credentials/server behavior are environment-specific.
Completed evidence: `docs/qa-ui-states.md` now includes a live SMTP smoke checklist for safe test sends, To/Cc/Bcc delivery, attachment sends, and retry behavior.
Verification: docs diff only; no credentials used.

### 076. Add session expiry manual QA checklist
Status: Completed
Evidence: Backend route-level expired sessions are tested; frontend 401 returns login.
Completed evidence: `docs/qa-ui-states.md` now includes a session-expiry smoke checklist for cookie expiry/removal, protected reload, and API-backed action behavior.
Verification: docs diff only.

### 077. Add accessibility pass for keyboard shortcuts
Status: Completed
Evidence: App supports `/`, `c`, `r`, `x`, `j/k`, `Escape`.
Completed evidence: `docs/qa-ui-states.md` now requires shortcut review and confirms shortcuts must not fire inside inputs, selects, textareas, or editable content.
Verification: docs diff only.

### 078. Add accessibility pass for icon-only buttons
Status: Completed
Evidence: Many actions are icon buttons with `aria-label`.
Completed evidence: `docs/qa-ui-states.md` now requires icon-only controls to have meaningful `aria-label` text and visible focus states.
Verification: docs diff only.

### 079. Add color contrast review
Status: Completed
Evidence: UI uses muted grays and accent colors for dense operational layout.
Completed evidence: `docs/qa-ui-states.md` now includes manual contrast review for muted text, badges, disabled controls, selected rows, and accent buttons.
Verification: docs diff only; no tooling dependency added.

### 080. Add mobile overflow QA for small devices below 375px
Status: Completed
Evidence: QA viewports specify 375x812 only.
Completed evidence: `docs/qa-ui-states.md` now requires a 320px-width mobile overflow review for inbox, reading pane, folder drawer, and compose.
Verification: docs diff only.

### 081. Add desktop wide-screen QA
Status: Completed
Evidence: QA viewports specify 1440x900 only.
Completed evidence: `docs/qa-ui-states.md` now requires wide desktop review such as `1920x1080` for reading content and compose placement.
Verification: docs diff only.

### 082. Add production smoke status recording
Status: Completed
Evidence: Deploy workflow runs smoke tests, but docs do not record latest smoke result.
Completed evidence: `docs/qa-ui-states.md` now defines production smoke recording fields and includes a deferred log row stating no deploy workflow or production check was run in this batch.
Verification: docs diff only; no deployment action run.

### 083. Add frontend test framework decision
Status: Approval-blocked
Evidence: `package.json` has no Vitest/Testing Library/Cypress/Playwright dependency.
Deferred rationale: frontend test dependency approval was not granted in the request matrix; no test framework dependency was added.
Verification: `package.json` unchanged for test tooling.

### 084. Add browser automation decision for visual QA
Status: Approval-blocked
Evidence: QA screenshots are deferred because no browser automation dependency exists in the workspace.
Deferred rationale: browser automation dependency approval was not granted in the request matrix; QA docs retain manual/browser-agent flow without adding Playwright or Puppeteer.
Verification: `package.json` unchanged for browser automation tooling.

Section: Documentation / Release Hygiene / Operations

### 085. Update README with latest release tag example
Status: Completed
Evidence: README deploy section still shows `joomail-v0.1.0` as the example.
Completed evidence: README deploy example now uses `joomail-v0.1.10`.
Verification: docs diff only.

### 086. Add release checklist document
Status: Completed
Evidence: Deployment is tag-triggered and smoke-tested, but release steps are only partially in README.
Completed evidence: `docs/release-checklist.md` covers pre-release verification, explicit approval before tags/deploys, release tag usage, deploy watch, smoke recording, and failure handling.
Verification: docs diff only.

### 087. Add rollback procedure document
Status: Approval-blocked
Evidence: Deploy workflow keeps `${JOOMAIL_STATIC_PATH}.prev` but no repo doc explains rollback.
Deferred rationale: 운영 rollback 문서 작성 approval was not granted in the request matrix, so rollback procedure details were not added.
Verification: no rollback docs added.

### 088. Address GitHub Actions Node.js 20 deprecation annotation
Status: Approval-blocked
Evidence: Deploy run reported Node.js 20 deprecation annotation from action runtimes.
Deferred rationale: CI/deploy workflow modification approval was not granted, and no deployment check was run in this batch.
Verification: `.github/workflows` untouched.

### 089. Add environment variable reference without values
Status: Completed
Evidence: README lists env var names but not validation semantics.
Completed evidence: README now describes required/optional IMAP, SMTP, login, session, and credential environment variables without values.
Verification: docs diff only.

### 090. Document IMAP/SMTP TLS modes
Status: Completed
Evidence: Config supports IMAP TLS, SMTP TLS, STARTTLS, and implicit TLS.
Completed evidence: README documents IMAP implicit TLS, SMTP implicit TLS, port 465 implicit TLS behavior, and SMTP STARTTLS mode.
Verification: docs diff only.

### 091. Document credential file lifecycle
Status: Completed
Evidence: Credential store saves encrypted credentials per session and deletes on logout.
Completed evidence: README explains encrypted per-session credential files are created after IMAP login, used for live IMAP/SMTP sessions, and deleted on logout; it also warns not to commit credentials, keys, secrets, or env values.
Verification: docs diff only.

### 092. Add source-vs-doc audit cadence
Status: Completed
Evidence: `docs/development-checklist.md` is the current implementation ledger.
Completed evidence: `docs/release-checklist.md` requires confirming `docs/development-checklist.md` synchronization before release, and the checklist maintenance rules continue to define per-batch source-vs-doc updates.
Verification: docs diff only.

### 093. Add changelog or release notes decision
Status: Completed
Evidence: Releases are versioned by tag, but no changelog file is present.
Completed evidence: `docs/release-checklist.md` records the decision to avoid a maintained changelog in the current phase and derive release notes from tags, PR/commit summary, and checklist evidence.
Verification: docs decision; no code tests.

### 094. Add health endpoint response contract note
Status: Completed
Evidence: `GET /api/health` returns `{"status":"ok"}`.
Completed evidence: README now documents `GET /api/health` returning `{"status":"ok"}` for smoke checks.
Verification: docs diff only.

Section: Non-goal Guardrails

### 095. Keep unified inbox excluded
Status: Non-goal guardrail
Evidence: `docs/webmail-ui-plan.md` explicitly excludes unified inbox and account-wide unified search.
Expected outcome: Future tasks do not add cross-account product flows without a separate product decision.
Verification: Preserved as a non-goal guardrail; no implementation added.

### 096. Keep conversation threading excluded
Status: Non-goal guardrail
Evidence: Plan excludes thread view and keeps IMAP folders as flat lists.
Expected outcome: Do not add References/In-Reply-To thread grouping in current phase.
Verification: Preserved as a non-goal guardrail; no implementation added.

### 097. Keep labels and rules excluded
Status: Non-goal guardrail
Evidence: Plan excludes labels/tags and rules/filters.
Expected outcome: Organizing behavior remains folder-based unless product scope changes.
Verification: Preserved as a non-goal guardrail; no implementation added.

### 098. Keep scheduled send and undo send excluded
Status: Non-goal guardrail
Evidence: Plan excludes scheduled send and undo send.
Expected outcome: Compose work stays limited to immediate SMTP send and approved Drafts behavior.
Verification: Preserved as a non-goal guardrail; no implementation added.

### 099. Keep contacts and calendar excluded
Status: Non-goal guardrail
Evidence: Plan excludes contacts/address book and calendar integration.
Expected outcome: Recipient UX does not expand into a contacts subsystem without approval.
Verification: Preserved as a non-goal guardrail; no implementation added.

### 100. Keep dark-mode toggle UI excluded
Status: Non-goal guardrail
Evidence: Plan allows token planning but excludes the dark-mode switch UI in this phase.
Expected outcome: Do not add a dark-mode toggle unless the product plan changes.
Verification: Preserved as a non-goal guardrail; no implementation added.

## Notes

- Items marked `Needs approval` must stop before implementation and ask the user.
- Persistence, database, background sync/indexing, CI/deploy workflow changes, Dovecot/Postfix configuration, new dependencies, and production operational changes are approval-gated by project rules.
- The backlog intentionally includes guardrails so future planning does not reintroduce explicitly excluded product features.
- Visual QA screenshots remain pending until a manual/browser-agent pass records results in `docs/qa-ui-states.md`.

## Verification

- Run `git diff --check -- docs/future-work-100.md`.
- Confirm exactly 100 item headings:
  `rg '^### [0-9]{3}\\.' docs/future-work-100.md | wc -l`
- This is a docs-only file; do not run Go or npm tests unless code is changed.
