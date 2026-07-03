# QA UI states

Use this checklist when capturing visual QA screenshots for development-only UI state routes.

## Setup

1. Run `npm run dev -- --host 127.0.0.1`.
2. Open the local URL printed by Vite. The default is `http://127.0.0.1:5173`; if that port is busy, use the next printed port.
3. Save screenshots under `docs/qa-screenshots/` or another review folder agreed for the QA pass. Do not commit generated screenshots unless the reviewer asks for them.
4. Capture each route at both viewport sizes:
   - Desktop: `1440x900`
   - Mobile: `375x812`

## Verification Commands

- Confirm this backlog still has exactly 100 numbered items:
  ```sh
  rg '^### [0-9]{3}\.' docs/future-work-100.md | wc -l
  ```
- Run `git diff --check` after docs-only QA updates.
- Run automated local QA route capture when browser tooling is installed:
  ```sh
  npm run qa:visual
  ```

## Screenshot Storage Policy

- Default local path: `docs/qa-screenshots/YYYY-MM-DD/`.
- Use a separate folder per QA pass and keep filenames from the route table below.
- Do not commit generated screenshots unless the reviewer explicitly asks for them.
- If screenshots are too large for the repo, store them in the review artifact location agreed for that pass and record the path or link in the QA Results Log.

## Routes

- `/?qa=normal`
- `/?qa=loading`
- `/?qa=error`
- `/?qa=empty`
- `/?qa=search`
- `/?qa=search-account`
- `/?qa=search-empty`
- `/?qa=multiselect`
- `/?qa=compose`
- `/?qa=remote-images-shown`
- `/?qa=quoted-expanded`
- `/?qa=long-overflow`
- `/?qa=many-attachments`
- `/?qa=empty-custom-folder`
- `/?qa=nested-tree`
- `/?qa=mobile-reading-attachments`
- `/?qa=compose-cc-bcc`

Use these screenshot names:

| Route | Desktop file | Mobile file |
|---|---|---|
| `/?qa=normal` | `desktop-normal.png` | `mobile-normal.png` |
| `/?qa=loading` | `desktop-loading.png` | `mobile-loading.png` |
| `/?qa=error` | `desktop-error.png` | `mobile-error.png` |
| `/?qa=empty` | `desktop-empty.png` | `mobile-empty.png` |
| `/?qa=search` | `desktop-search.png` | `mobile-search.png` |
| `/?qa=search-account` | `desktop-search-account.png` | `mobile-search-account.png` |
| `/?qa=search-empty` | `desktop-search-empty.png` | `mobile-search-empty.png` |
| `/?qa=multiselect` | `desktop-multiselect.png` | `mobile-multiselect.png` |
| `/?qa=compose` | `desktop-compose.png` | `mobile-compose.png` |
| `/?qa=remote-images-shown` | `desktop-remote-images-shown.png` | `mobile-remote-images-shown.png` |
| `/?qa=quoted-expanded` | `desktop-quoted-expanded.png` | `mobile-quoted-expanded.png` |
| `/?qa=long-overflow` | `desktop-long-overflow.png` | `mobile-long-overflow.png` |
| `/?qa=many-attachments` | `desktop-many-attachments.png` | `mobile-many-attachments.png` |
| `/?qa=empty-custom-folder` | `desktop-empty-custom-folder.png` | `mobile-empty-custom-folder.png` |
| `/?qa=nested-tree` | `desktop-nested-tree.png` | `mobile-nested-tree.png` |
| `/?qa=mobile-reading-attachments` | `desktop-mobile-reading-attachments.png` | `mobile-mobile-reading-attachments.png` |
| `/?qa=compose-cc-bcc` | `desktop-compose-cc-bcc.png` | `mobile-compose-cc-bcc.png` |

## Manual browser steps

For each route:

1. Open DevTools responsive mode or the browser-agent viewport control.
2. Set viewport to `1440x900`, navigate to the route, wait for the UI to settle, and save the desktop screenshot.
3. Set viewport to `375x812`, navigate to the same route, wait for the UI to settle, and save the mobile screenshot.
4. Reload between route changes if state from the previous route remains visible.

## Browser-agent prompt

Use this prompt when driving a browser-agent manually:

```text
Open the JooMail dev server. For each route in docs/qa-ui-states.md, capture screenshots at 1440x900 and 375x812 using the documented filenames. Wait for the UI to settle after navigation. Do not click the DEV QA state switcher; use the query-string routes.
```

## Review checklist

- Desktop keeps the three-pane layout at `1440x900`.
- Tablet widths from `768px` through `1279px` keep the compact sidebar rail, message list, and reading pane visible without clipped toolbar controls.
- Mobile uses the single-column inbox or full-screen compose at `375x812`.
- Loading, error, empty, and search-empty states stay inside the local list or reading pane area.
- Search state highlights matching text in subject or snippet.
- Account-scope search shows the "현재 계정" scope selected on desktop and mobile.
- Account-scope search rows show mailbox labels, and capped account results show `최신 50건` when the visible count reaches the live cap.
- Search input remains responsive while results update after the debounce; clearing search resets to current-mailbox scope and clears selected rows.
- Multiselect state shows the selected-count bar and checked rows without overlapping unread dots or avatars.
- Shift-click selects a contiguous desktop message range from the last checked row; Cmd/Ctrl-click toggles one row without clearing the rest of the selection.
- Desktop and mobile message rows keep checkbox, unread dot, avatar, sender, subject, and snippet anchors fixed between normal, selected, hover, and multiselect states.
- Desktop hover/focus row actions for archive, trash, and mark unread appear without shifting sender, timestamp, subject, snippet, attachment, or star controls.
- Long sender and subject state truncates or wraps inside its container without covering timestamps, icons, or list controls.
- Nested tree state shows custom folder hierarchy with readable indentation on desktop and in the mobile drawer.
- Compose state does not overlap controls; textarea focus does not draw an oversized blue outline.
- Compose `From` opens the account menu, Cc/Bcc expands local input rows, and the paperclip button displays selected file names and sizes before send.
- Compose Cc/Bcc route opens those fields by default and keeps mobile controls reachable.
- Compose close, delete, Escape, and mobile/browser back confirm before discarding dirty unsent content; canceling keeps the compose panel open.
- Compose attachment chips show per-file remove controls, update the aggregate attachment size, and removed files are not sent.
- Compose send stays disabled until recipients and subject are present, with the missing required field visible in the footer/title.
- Compose send failure leaves recipients, subject, body, and selected attachments intact and changes the send button to a retry label.
- Reading pane recipient details, remote-image display, and quoted conversation controls toggle local UI state only.
- Remote-image displayed route shows the displayed state directly without clicking.
- Quoted-expanded route shows the quoted block directly without clicking.
- Many-attachments route keeps attachment chips wrapping within the reading pane.
- Mobile reading attachments route opens the mobile reading view with attachment rows visible.
- Reply opened with `r` keeps the recipient chip and `Re:` subject.
- Global shortcuts are ignored while an input, select, or textarea has focus; `c`, `r`, `x`, `j/k`, and `Escape` keep the normal inbox behavior.
- Reply, reply-all, forward, compose header, and list controls keep even spacing.
- Selected account, mailbox, message, and search scope restore from `joomail:mail-state` after reload without opening the wrong message.
- Keyboard shortcuts remain discoverable in this checklist and never fire while focus is inside inputs, selects, textareas, or editable content.
- Icon-only controls have meaningful `aria-label` text and visible focus states.
- Muted text, badges, disabled controls, selected rows, and accent buttons remain readable against their backgrounds.
- Small mobile width `320px` does not overlap critical controls in the inbox, reading pane, folder drawer, or compose panel.
- Wide desktop layouts such as `1920x1080` do not over-stretch reading content or misplace compose.

## Live Smoke Checklists

Use live smoke checks only against an approved test mailbox. Do not record credentials, server secrets, or message contents in this document.

### IMAP Smoke

- Login with a real IMAP-backed account succeeds.
- Account and mailbox list loads from live IMAP data.
- Nested/selectable mailboxes can be opened.
- Message list opens newest live messages without mock data.
- Message detail renders parsed text or sanitized HTML from the backend response.
- Attachment download succeeds for a safe test attachment.
- Logout clears the session and returns to login.

### SMTP Smoke

- Send a small test message from an approved test account to an approved recipient.
- Verify To/Cc delivery as applicable.
- Verify Bcc delivery without a visible Bcc message header.
- Verify a small safe attachment can be sent.
- Verify a failed send keeps compose fields and attachments available for retry.

### Session Expiry Smoke

- Expire or remove the session cookie in a controlled browser session.
- Reload a protected route and confirm the app returns to login.
- Trigger an API-backed action after expiry and confirm the UI does not remain in a broken authenticated state.

### Production Smoke Recording

- Record the deploy run URL or release identifier only after an approved deploy.
- Record smoke result, date, tester, and blockers in the QA Results Log.
- Do not run deploy workflows or modify deployment state from this checklist.

## QA Results Log

| Date | Viewports | Routes | Screenshot location | Result | Notes |
|---|---|---|---|---|---|
| 2026-07-03 | Not run | Deployed `joomail-v0.1.9` | Pending | Deferred | Deployed visual QA was not executed in this workspace; run the route table against the approved deployed URL before release sign-off. |
| 2026-07-03 | Not run | Live IMAP/SMTP smoke | Pending | Deferred | Live smoke requires approved test credentials and must not record secrets. Use the smoke checklists above. |
| 2026-07-03 | Not run | Production smoke | Pending | Deferred | Production smoke recording is documented, but no deploy workflow or production check was run in this batch. |
| 2026-07-03 | Pending | All documented routes | `docs/qa-screenshots/YYYY-MM-DD/` | Ready | Browser automation is now wired through `npm run qa:visual`; generated screenshots stay ignored unless a reviewer explicitly asks to commit them. |
| 2026-07-03 | 1440x900, 375x812 | 17 QA routes x 2 viewports | `docs/qa-screenshots/2026-07-03/` | Pass | `npm run qa:visual` passed 34 screenshots locally with Playwright Chromium. |
