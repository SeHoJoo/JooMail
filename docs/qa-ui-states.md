# QA UI states

Use this checklist when capturing visual QA screenshots for the mock UI state routes.

## Setup

1. Run `npm run dev -- --host 127.0.0.1`.
2. Open the local URL printed by Vite. The default is `http://127.0.0.1:5173`; if that port is busy, use the next printed port.
3. Capture each route at both viewport sizes unless noted:
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

For the focused mobile pass, capture:

- `/`
- `/?qa=loading`
- `/?qa=error`
- `/?qa=empty`
- `/?qa=search-empty`
- `/?qa=compose`

## Browser-agent steps

For each route:

1. Set viewport to `1440x900`, navigate to the route, wait for the UI to settle, and save a screenshot named `desktop-<state>.png`.
2. Set viewport to `375x812`, navigate to the route, wait for the UI to settle, and save a screenshot named `mobile-<state>.png`.
3. Check that no controls overlap, list rows remain readable, and loading/error/empty states stay inside the local list or reading pane areas.

Use `normal` for `/` when naming files.
