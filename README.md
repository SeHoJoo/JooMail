# JooMail

## Backend

Run the mock Go API server:

```sh
go run ./cmd/joomaild
```

The server listens on `127.0.0.1:8080` by default. Set `JOOMAIL_ADDR` to override it.

Initial mock endpoints:

- `GET /api/health`
- `GET /api/accounts`
- `GET /api/accounts/{accountID}/mailboxes/{mailboxID}/messages`
- `GET /api/messages/{messageID}`
