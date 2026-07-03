# JooMail Rollback

Use this runbook only after an explicit production rollback approval.

## Scope

- This does not change Dovecot, Postfix, DNS, TLS, reverse proxy, firewall, or secrets.
- This assumes the deploy workflow already created `/opt/JooMail/www.prev`.
- Backend binary rollback requires an approved artifact or tag-specific rebuild; the current workflow only keeps the previous static frontend directory automatically.

## Static Frontend Rollback

1. SSH to the production host with the approved operator account.
2. Verify the previous frontend directory exists:
   ```sh
   sudo -n test -d /opt/JooMail/www.prev
   ```
3. Swap the current and previous frontend directories:
   ```sh
   sudo -n mv /opt/JooMail/www /opt/JooMail/www.bad
   sudo -n mv /opt/JooMail/www.prev /opt/JooMail/www
   sudo -n chown -R joomail:joomail /opt/JooMail/www
   sudo -n systemctl restart joomail
   ```
4. Smoke check the local service:
   ```sh
   curl -fsS http://127.0.0.1:8081/api/health
   curl -fsS http://127.0.0.1:8081/
   ```
5. Keep `/opt/JooMail/www.bad` until the failed release is reviewed.

## Backend Rollback

1. Choose the approved rollback source: a known-good tag or a preserved binary artifact.
2. Rebuild or fetch the known-good `joomaild` binary outside this runbook's text, then install it atomically:
   ```sh
   sudo -n install -o root -g root -m 0755 /tmp/joomaild /opt/JooMail/bin/joomaild.new
   sudo -n mv /opt/JooMail/bin/joomaild.new /opt/JooMail/bin/joomaild
   sudo -n systemctl restart joomail
   ```
3. Smoke check `/api/health`, the login page, and one approved live IMAP login.

Record the rollback date, operator, release/tag, smoke result, and blockers in `docs/qa-ui-states.md` or the external incident record. Do not record credentials, session cookies, or message contents.
