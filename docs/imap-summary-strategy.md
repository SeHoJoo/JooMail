# IMAP Summary Fetch Strategy

Current message summaries intentionally keep using `BODY.PEEK[]` for the newest
matching UIDs.

## Decision

Do not switch summaries to envelope-only `UID FETCH` yet.

The current summary response depends on backend MIME parsing for decoded headers,
snippet generation, attachment presence, HTML/text fallback behavior, and stable
public JSON fields. An envelope-only fetch would reduce bandwidth, but it would
also require a bodystructure-aware summary parser or a changed response contract.

## Future Path

- Preserve existing `messages` field names.
- Add an internal summary parser that can derive sender, subject, date, flags,
  and attachment presence from `ENVELOPE`, `FLAGS`, and `BODYSTRUCTURE`.
- Fetch full MIME only when opening a message detail or when a summary field
  cannot be derived without degrading current behavior.
- Cover the change with fake IMAP command tests before using it against live
  mailboxes.
