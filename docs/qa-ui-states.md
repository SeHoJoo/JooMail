# QA UI states

Use this checklist when capturing visual QA screenshots for the mock UI state routes.

## Setup

1. Run `npm run dev -- --host 127.0.0.1`.
2. Open the local URL printed by Vite. The default is `http://127.0.0.1:5173`; if that port is busy, use the next printed port.
3. Save screenshots under `docs/qa-screenshots/` or another review folder agreed for the QA pass. Do not commit generated screenshots unless the reviewer asks for them.
4. Capture each route at both viewport sizes:
   - Desktop: `1440x900`
   - Mobile: `375x812`

## Routes

- `/`
- `/?qa=loading`
- `/?qa=error`
- `/?qa=empty`
- `/?qa=search`
- `/?qa=search-empty`
- `/?qa=multiselect`
- `/?qa=compose`

Use these screenshot names:

| Route | Desktop file | Mobile file |
|---|---|---|
| `/` | `desktop-normal.png` | `mobile-normal.png` |
| `/?qa=loading` | `desktop-loading.png` | `mobile-loading.png` |
| `/?qa=error` | `desktop-error.png` | `mobile-error.png` |
| `/?qa=empty` | `desktop-empty.png` | `mobile-empty.png` |
| `/?qa=search` | `desktop-search.png` | `mobile-search.png` |
| `/?qa=search-empty` | `desktop-search-empty.png` | `mobile-search-empty.png` |
| `/?qa=multiselect` | `desktop-multiselect.png` | `mobile-multiselect.png` |
| `/?qa=compose` | `desktop-compose.png` | `mobile-compose.png` |

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
- Mobile uses the single-column inbox or full-screen compose at `375x812`.
- Loading, error, empty, and search-empty states stay inside the local list or reading pane area.
- Search state highlights matching text in subject or snippet.
- Multiselect state shows the selected-count bar and checked rows without overlapping unread dots or avatars.
- Desktop and mobile message rows keep checkbox, unread dot, avatar, sender, subject, and snippet anchors fixed between normal, selected, hover, and multiselect states.
- Compose state does not overlap controls; textarea focus does not draw an oversized blue outline.
- Compose `From` opens the mock account menu, Cc/Bcc expands local input rows, and the paperclip button displays selected file names and sizes without uploading.
- Reading pane recipient details, remote-image display, and quoted conversation controls toggle local UI state only.
- Reply opened with `r` keeps the recipient chip and `Re:` subject.
- Global shortcuts are ignored while an input, select, or textarea has focus; `c`, `r`, `x`, `j/k`, and `Escape` keep the normal inbox behavior.
- Reply, reply-all, forward, compose header, and list controls keep even spacing.
