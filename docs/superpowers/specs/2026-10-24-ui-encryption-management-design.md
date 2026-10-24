# UI Encryption Management — Design

Date: 2026-10-24
Status: Draft, awaiting approval

## Intent

Database encryption (AES-256-GCM over SMTP passwords, SMS provider configs,
IMAP passwords and captured `Event.Details`) is today operated entirely through
CLI flags and an environment variable set before startup. Admins should be able
to manage it from inside the platform: generate a key, turn encryption on,
see status, preview and run migrations, and turn it off — without CLI flags and
without restarting the service to turn it on.

The key's long-term home stays outside the platform: the
`ANGLERPHISH_ENCRYPTION_KEY` environment variable, set by hand or injected by a
KMS / secret manager (Vault Agent, Azure Key Vault, AWS Secrets Manager, Docker
or Kubernetes secrets). The platform never persists the key.

Why: the encryption protects against a leaked `gophish.db` or backup. Anything
that stores the key next to the data (DB, `config.json`, a key file in the app
folder) defeats that, so the key stays in the environment / secret manager.

## Goals

- Every capability of the current CLI flags is available from an admin-only UI:
  key generation, status, dry-run preview, encrypt existing data, decrypt
  existing data.
- An admin can turn encryption on in one sitting, no restart.
- If the platform starts without the right key while encrypted data exists, it
  stays up in a safe locked state and can be recovered from the UI without a
  restart.
- Losing the key through a careless click is prevented.

## Non-goals

- Native KMS integration (KMS works by injecting the env var).
- Key rotation.
- A restart button in the platform.
- Storing the key on disk, in `config.json`, or in the database.

## Anti-goals (failure even if it "works")

- An `ENC:v1:...` string is ever used as an SMTP/IMAP/SMS credential.
- The key appears in logs, audit entries, API responses after the reveal
  dialog, or browser caches.
- Disabling encryption leaves rows encrypted with no key to read them.
- A non-admin can see or operate any of this.

## Constraints

- Existing per-value coexistence stays: values carry an `ENC:v1:` prefix;
  `Decrypt` passes plaintext through; `Encrypt` and the migrations skip values
  already in the target state. Mixed plaintext/encrypted data is a normal state.
- `crypto` package stays standard-library only (`crypto/crypto.go:1-3`).
- Both SQLite and MySQL are supported; any schema change needs a migration in
  `db/db_sqlite3/migrations` and `db/db_mysql/migrations`.
- Admin gating uses the existing `models.PermissionModifySystem` and
  `mid.RequirePermission(...)` pattern (see `controllers/api/server.go:104-117`).
- Admin server already runs with TLS (`admin_server.use_tls: true`).

## Key sources

1. **Environment** — `ANGLERPHISH_ENCRYPTION_KEY` read at startup (unchanged
   format: base64url, 32 bytes, 44 chars).
2. **Session** — a key generated or pasted in the UI, held in memory only until
   the process exits.

The environment wins at startup. The UI always shows which source the active
key came from.

## States and UI

A new **Encryption** tab in Settings, rendered only when `ModifySystem` is true
(same as other admin-only tabs).

| State | Condition | UI |
|---|---|---|
| **Off** | No key loaded, no encrypted values in DB | Explanation + *Generate key* |
| **On (session)** | Key loaded from UI this session | Status + actions + persistent warning banner (all admin pages): "Encryption key is active for this session only. Add it to your environment or secret manager before the next restart." |
| **On (environment)** | Key loaded from env var | Status + actions, no banner |
| **Locked** | Encrypted values exist but no key, or key fails the check value | Red banner on all admin pages; sending blocked; *Unlock* field to paste key |
| **Disabling** | Decrypt in progress | Progress / result, then instruction to remove the env var |

"Status + actions" = per-field-type counts (encrypted / plaintext / empty /
total, from the existing `EncryptionReport`), *Preview encryption* and
*Encrypt existing data*, *Preview decryption* and *Disable encryption*.

### Turn on (generate)

1. Admin clicks *Generate key*. Server generates the key with
   `crypto.GenerateEncryptionKey()` and returns it **once**; it is not loaded yet
   and not retained server-side after the response.
2. Dialog shows the key with a copy button and instructions for env / secret
   manager.
3. Admin must **paste the key back** to proceed (proves a copy exists). A
   checkbox is not enough.
4. Server validates format, loads it as the session key, writes the check
   value, and encryption is on: new writes are encrypted from this point.
5. Existing data is **not** migrated automatically. Admin runs *Preview
   encryption* then *Encrypt existing data* as a separate deliberate step.

If the env var is already set, *Generate key* is not offered; the state is
**On (environment)** from startup.

### Unlock

Paste key → server validates format and the check value → on success loads it
as the session key, leaves Locked, unblocks sending. On failure: generic error,
key not logged. Unlock attempts are rate-limited per user session (builder
picks the limit).

### Disable

1. Admin clicks *Disable encryption*, sees decrypt preview, confirms.
2. Stop encrypting new writes while keeping the key loaded for reads
   (a "decrypt-only" mode in `crypto`).
3. Run `MigrateToDecrypted()`.
4. If zero encrypted values remain: unload key, remove the check value, state
   becomes **Off**, UI tells admin to remove the env var / secret.
   If any remain (errors): stay in decrypt-only mode, show the failures, allow
   retry. Never unload the key while encrypted values remain.

If the env var is still set at the next restart after disabling, the platform
loads it and returns to **On (environment)** — that is expected; the UI message
in step 4 explains it.

## Check value

A small known value encrypted with the active key, stored in the DB, used to
tell "wrong key" from "no key" at startup and on unlock.

- Stored in a new single-row table (name left to builder, e.g.
  `encryption_meta`) with a migration for SQLite and MySQL.
- Written when encryption is first turned on (generate, or first startup with
  an env key and no existing check value).
- At startup with an env key: if a check value exists and fails to decrypt →
  **Locked** (wrong key), and the env key is **not** used for writes.
- Removed when disabling completes.
- Existing deployments that already have encrypted data but no check value:
  first startup with an env key writes one, *only if* that key successfully
  decrypts at least one existing encrypted value (or there are none).
  Otherwise → Locked.

## Locked mode: blocking credential use

While Locked, nothing may use a credential that is still encrypted:

- Email sending: `SMTP.GetDialer` (`models/smtp.go:112`) and the paths through
  `MailLog.GetDialer` / `EmailRequest.GetDialer`.
- SMS sending: `SMSLog.GetDialer` / `SMSRequest.GetDialer`.
- IMAP polling: `imap/monitor.go:416`, `imap/imap.go:68`.

Rule: if a credential value is still `ENC:v1:` after `AfterFind`, the consumer
returns an error ("encryption key unavailable") instead of using it. Today
`AfterFind` falls back to the raw value on failure (`models/smtp.go:277-283`);
that fallback must no longer reach a dialer. Failed sends in Locked mode go
through the normal error/retry handling; the builder decides whether to pause
the worker or let maillogs error and back off, as long as no encrypted string
is sent.

Scope of the block is **per value**, not global: profiles whose credential is
plaintext keep working while Locked; only consumers holding a still-encrypted
credential refuse. (Designer's call — consistent with coexistence and the
anti-goal, which is about never sending an `ENC:v1:` string.)

Captured data in Locked mode: `Event.AfterFind` keeps clearing unreadable
`Details` to `""` as today (so JSON consumers — timeline, `GetReplies`, CSV
export, webhooks — don't break), and a new non-persisted field on `Event`
(e.g. `details_locked: true`) marks it. The admin web UI shows "encrypted —
unlock to view" where the flag is set; exports and webhooks get the empty value
plus the flag. (Designer's call.)

Non-admin users see a generic notice in the base layout while Locked
("Some sending is unavailable — contact an administrator"), driven by a
boolean in template params. They see no key source, status, or controls.
(Designer's call.)

### Writes while Locked

New sensitive writes (captured `Event.Details`, SMTP/SMS/IMAP profile
credentials) are **saved as plaintext**, exactly as `Encrypt` behaves today
with no key loaded. Nothing is refused or lost. The Locked banner shows how
many sensitive values are currently plaintext (from the status report), and
after unlock the admin runs *Encrypt existing data* to cover them; the
session/unlock UI prompts for this when plaintext count > 0. (Decided by the
user.)

### Discard encrypted data (lost key)

A destructive admin action available only in **Locked** state, for when the
key is truly lost:

1. Admin opens *Discard encrypted data*; UI lists what will be lost (counts
   per field type from the status report) and states that affected SMTP, SMS
   and IMAP credentials must be re-entered and that captured data will be gone.
2. Admin must type a confirmation phrase (builder picks the wording, e.g.
   `DISCARD ENCRYPTED DATA`).
3. Server blanks every value still carrying the `ENC:v1:` prefix (SMTP
   password, SMS provider config, IMAP password, `Event.Details`), removes the
   check value, and the state becomes **Off**.
4. Audit entry records who, when and per-type counts.

If a (wrong) env key is still set, the next restart finds no check value and no
encrypted values, so by the check-value rules that env key is accepted and
becomes the new key. The completion message tells the admin to either remove
the env var or keep it knowingly as the new key. (Decided by the user.)

API: `POST /api/encryption/discard` — body: confirmation phrase.

## Concurrency

`encryptionKey` / `encryptionEnabled` become mutex-guarded (`sync.RWMutex`)
with a mode enum (off / on / decrypt-only). All reads go through accessors.
Migrations may run while campaigns run; coexistence makes this safe for
enable. Only one migration at a time (server rejects a second while one runs).

## Key handling rules

- Key appears in exactly one API response (generate). Responses carrying it
  set `Cache-Control: no-store`.
- Never logged, never in audit entries, never echoed in errors.
- Paste-back and unlock submit over POST body only.
- Audit log (via existing `log.Infof` style) records who did what and when:
  generated, enabled, unlock success/failure, migration started/finished with
  counts, disabled. Never the key.

## API (shape left to builder, admin-only, CSRF/API-key per existing pattern)

- `GET  /api/encryption/status` — state, key source, per-field report,
  migration-in-progress flag.
- `POST /api/encryption/generate` — returns new key (not loaded).
- `POST /api/encryption/enable` — body: key (paste-back). Loads as session key.
- `POST /api/encryption/unlock` — body: key.
- `POST /api/encryption/migrate` — body: `{direction: encrypt|decrypt, dry_run: bool}`.
- `POST /api/encryption/disable` — runs the disable sequence.

## Removals and docs

- Remove `--generate-encryption-key`, `--migrate-encryption`,
  `--migrate-decryption`, `--encryption-status`, `--dry-run` and their
  handlers in `gophish.go`. Nothing else references them (Dockerfile,
  `docker/run.sh`, Ansible checked).
- Rewrite the encryption section of `templates/api_documentation.html`
  (~lines 1099-1216) for the UI flow + env/secret-manager injection examples
  (systemd `EnvironmentFile`/`LoadCredential`, Docker secrets, Vault Agent).
- Update `FEATURES.md:71` and add a `CHANGELOG.md` entry.

## Decided in conversation

- Key long-term source: env var only, set by hand or by a KMS/secret manager.
- Generate in UI turns encryption on immediately (no restart), guarded by
  paste-back confirmation.
- Paste-to-unlock for missing/wrong key at startup.
- No restart button; no native KMS; no key rotation; no key on disk.
- CLI flags removed, replaced by UI.
- Writes while Locked are saved as plaintext and covered by a later
  *Encrypt existing data*.
- A destructive *Discard encrypted data* action exists for a truly lost key.

## Calls made by the designer (check these)

- Generate does not auto-migrate existing data; migration is a separate step.
- Check value lives in a new DB table.
- Locked mode blocks sending/IMAP rather than refusing to start.
- Session-key banner shown on all admin pages, not only Settings.
- Env key takes precedence over any session key at startup (there is no
  persisted session key anyway).
- Locked-mode blocking is per credential value, not global.
- Unreadable captured data stays `""` in `Details` plus a `details_locked`
  flag; only the admin web UI renders a placeholder.
- Non-admins get a generic "sending unavailable" notice while Locked.

## Left to the builder

- Table/column naming, exact endpoints and JSON shapes.
- UI layout within the Settings tab, wording of messages.
- Unlock rate limit value.
- Whether Locked mode pauses the worker or lets sends fail and retry.
- Test strategy (unit tests for `crypto` modes and check value; API tests for
  permission gating and key-not-leaked; model tests for locked-mode dialer
  refusal).
