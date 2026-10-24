# UI Encryption Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the encryption CLI flags with an admin-only Settings UI that generates a key, turns encryption on without a restart, runs migrations, unlocks a locked platform, disables encryption, and discards data for a lost key.

**Architecture:** `crypto` gets a mutex-guarded key holder with modes (off / on / decrypt-only) and a key source (none / environment / session). A new `models/encryption_state.go` owns the state machine (startup rules, enable, unlock, migrate, disable, discard) and a check value stored in a new `encryption_meta` table. Credential consumers refuse any still-encrypted value. A new admin API (`controllers/api/encryption.go`) drives it; a new Settings tab and base-layout banners render it.

**Tech Stack:** Go 1.25, GORM v1 (jinzhu), goose SQL migrations (SQLite + MySQL), gocheck (`models`), stdlib `testing` (`crypto`, `controllers/api`), jQuery/Bootstrap 3 front end built with gulp/webpack.

**Spec:** `docs/superpowers/specs/2026-10-24-ui-encryption-management-design.md`

## Global Constraints

- Key format unchanged: base64url, 32 bytes, 44 chars; env var `ANGLERPHISH_ENCRYPTION_KEY`; ciphertext prefix `ENC:v1:`.
- `crypto` package stays standard-library only.
- The key is never persisted (DB, `config.json`, disk), never logged, never in audit entries, never echoed in errors; it appears only in the `generate` response, which carries `Cache-Control: no-store`.
- Every new API endpoint and the Settings tab are gated by `models.PermissionModifySystem`.
- Every schema change ships in both `db/db_sqlite3/migrations` and `db/db_mysql/migrations`.
- Locked-mode error text: `encryption key unavailable`.
- Discard confirmation phrase: `DISCARD ENCRYPTED DATA`.
- Commits: short subject line only, no body, no trailer. Never push. Work on branch `feat/ui-encryption-management` created from `master` before Task 1.
- Test commands run from the repo root. Race tests: `go test -race` (cgo/gcc is available — sqlite already builds).

## Review Focus

- **Malformed env key at startup** (wrong length, not base64url): must not `log.Fatal` as today; platform starts **Locked** (`reason=invalid_key`) if encrypted data exists, otherwise **Off** with the reason surfaced in status. → Task 3 `TestStartupMalformedEnvKey`.
- **Admin saves an SMTP/IMAP profile while Locked** (the loaded password is still `ENC:v1:...` because `AfterFind` falls back to raw): the save must store the ciphertext unchanged — not blank it, not double-encrypt — so unlocking later restores it. → Task 4 `TestSMTPSaveWhileLockedKeepsCiphertext`.
- **Events written during Disable** (campaign traffic between `SetDecryptOnly` and the end of migration): must be stored plaintext, and Disable must end with zero encrypted values. → Task 3 `TestDisableWithConcurrentWrites`.
- **Enable called when encrypted data exists but no key is loaded**: must be refused (that is Unlock's job) so a new key is never layered over ciphertext from an old one. → Task 3 `TestEnableRefusedWhenEncryptedDataExists`.
- **Legacy DB** (encrypted data from the CLI era, no check value) started with the correct env key → On and a check value is written; with a wrong env key → Locked `wrong_key`. → Task 3 `TestStartupLegacyNoCheckValue`.

---

## File Structure

| File | Responsibility |
|---|---|
| `crypto/crypto.go` (modify) | Key holder: modes, source, mutex, load/unload, decrypt-with-candidate-key |
| `crypto/crypto_test.go` (create) | Unit + race tests for the key holder |
| `db/db_sqlite3/migrations/20261024000001_add_encryption_meta.sql`, `db/db_mysql/migrations/20261024000001_add_encryption_meta.sql` (create) | `encryption_meta` table |
| `models/encryption_meta.go` (create) | Check value read/write/verify, encrypted-value counting and sampling |
| `models/encryption_state.go` (create) | State machine: startup, enable, unlock, migrate, disable, discard, status |
| `models/encryption_state_test.go` (create) | gocheck tests for the state machine |
| `models/encryption.go` (modify) | Replace string status with a report struct; keep migrations |
| `models/smtp.go`, `models/sms_request.go`, `models/smslog.go`, `models/campaign.go`, `imap/imap.go`, `imap/monitor.go` (modify) | Refuse encrypted credentials; `DetailsLocked` flag |
| `models/encryption_lock_test.go` (create) | Locked-mode refusal tests |
| `gophish.go` (modify) | Remove flags/handlers; call `models.InitEncryptionAtStartup()` |
| `controllers/api/encryption.go`, `controllers/api/encryption_test.go` (create); `controllers/api/server.go` (modify) | Admin API |
| `controllers/route.go`, `templates/base.html` (modify) | Banner flags + banners |
| `templates/settings.html`, `static/js/src/app/settings.js` (modify) | Encryption tab |
| `templates/api_documentation.html`, `FEATURES.md`, `CHANGELOG.md` (modify) | Docs |

---

### Task 1: Mutex-guarded key holder in `crypto`

**Files:**
- Modify: `crypto/crypto.go`
- Test: `crypto/crypto_test.go`

**Interfaces:**
- Produces:
  - `type Mode int` with `ModeOff`, `ModeOn`, `ModeDecryptOnly`
  - `type KeySource string` with `SourceNone = "none"`, `SourceEnv = "environment"`, `SourceSession = "session"`
  - `ValidateKey(key string) error` — returns `ErrInvalidKeyLength` on bad format
  - `ReadEnvKey() (key string, present bool)` — raw env value, no validation
  - `LoadKey(key string, source KeySource) error` — validates, sets key, mode `ModeOn`
  - `SetDecryptOnly()` — requires a loaded key; Encrypt passes through, Decrypt still works
  - `Unload()` — clears key, mode `ModeOff`, source `SourceNone`
  - `CurrentMode() Mode`, `CurrentSource() KeySource`, `HasKey() bool`
  - `IsEncryptionEnabled() bool` — kept, true only in `ModeOn`
  - `DecryptWithKey(ciphertext, key string) (string, error)` — decrypts with a candidate key without loading it
  - `GenerateEncryptionKey`, `Encrypt`, `Decrypt`, `IsEncrypted`, `GetEncryptionStatus` keep their signatures
- Removes: `InitEncryption()`, `InitEncryptionWithKey()` (only `gophish.go` calls them; Task 5 rewires it). Also `MustEncrypt`/`MustDecrypt`/`TestEncryptionKey` if `grep` shows no callers.

- [ ] **Step 1: Write the failing tests** in `crypto/crypto_test.go` (package `crypto`, stdlib `testing`, each test calls `Unload()` in a `t.Cleanup`):
  - `TestEncryptPassthroughWhenOff`: `Encrypt("x")` == `"x"`.
  - `TestRoundTripWhenOn`: `LoadKey(k, SourceSession)`; `c, _ := Encrypt("secret")`; `strings.HasPrefix(c, "ENC:v1:")`; `Decrypt(c)` == `"secret"`; `CurrentSource()` == `SourceSession`.
  - `TestDecryptOnlyMode`: load, encrypt `c`, `SetDecryptOnly()`; `Encrypt("new")` == `"new"`; `Decrypt(c)` == `"secret"`; `IsEncryptionEnabled()` false; `HasKey()` true.
  - `TestUnloadDropsKey`: after `Unload()`, `Decrypt(c)` returns `ErrNoKeyConfigured`; `CurrentMode()` == `ModeOff`.
  - `TestLoadKeyRejectsBadFormat`: `LoadKey("short", SourceSession)` and a 44-char non-base64url string both return `ErrInvalidKeyLength`; mode stays `ModeOff`.
  - `TestDecryptWithKey`: with key A loaded, encrypt `c`; `Unload()`; `DecryptWithKey(c, A)` == plaintext; `DecryptWithKey(c, B)` returns `ErrDecryptionFailed`; `HasKey()` still false.
  - `TestConcurrentLoadAndEncrypt`: 50 goroutines calling `Encrypt`/`Decrypt` while another toggles `LoadKey`/`SetDecryptOnly`/`Unload`; asserts nothing beyond no panic (the race detector is the assertion).

- [ ] **Step 2: Run to verify failure**
  Run: `go test ./crypto/ -v`
  Expected: FAIL — `undefined: LoadKey` (and friends).

- [ ] **Step 3: Implement** in `crypto/crypto.go`: one package-level struct `{mu sync.RWMutex; key []byte; mode Mode; source KeySource}`; every accessor takes the lock; `Encrypt` encrypts only in `ModeOn`; `Decrypt` works when a key is present (`ModeOn` or `ModeDecryptOnly`); share the GCM open logic between `Decrypt` and `DecryptWithKey`. Update the doc comment that mentions `--generate-encryption-key`.

- [ ] **Step 4: Run to verify pass**
  Run: `go test -race ./crypto/ -v`
  Expected: PASS, no `WARNING: DATA RACE`.

- [ ] **Step 5: Commit**
  ```bash
  git add crypto/
  git commit -m "feat: mutex-guarded encryption key holder with modes"
  ```
  (`gophish.go` will not compile until Task 5 — run `go build ./crypto/ ./models/` only. If the executor wants every commit buildable, temporarily replace the four `crypto.InitEncryption()` calls in `gophish.go` with a local shim calling `ReadEnvKey`+`LoadKey`; Task 5 deletes it.)

---

### Task 2: Check value table and helpers

**Files:**
- Create: `db/db_sqlite3/migrations/20261024000001_add_encryption_meta.sql`, `db/db_mysql/migrations/20261024000001_add_encryption_meta.sql`
- Create: `models/encryption_meta.go`
- Modify: `models/encryption.go` (status → struct), `models/models_test.go` (`TearDownTest` deletes `EncryptionMeta{}`)
- Test: `models/encryption_state_test.go` (start the file here)

**Interfaces:**
- Consumes: Task 1 (`Encrypt`, `DecryptWithKey`, `IsEncrypted`).
- Produces:
  - Table `encryption_meta(id, check_value TEXT NOT NULL, created_at)`; model `EncryptionMeta{Id int64; CheckValue string; CreatedAt time.Time}` with `TableName() "encryption_meta"`. Follow `20261013000001_add_tracked_requests.sql` format for both dialects.
  - `const encryptionCheckPlaintext = "anglerphish-encryption-check-v1"`
  - `writeCheckValue() error` — encrypts the constant with the loaded key, replaces any existing row. Errors if `!crypto.IsEncryptionEnabled()`.
  - `deleteCheckValue() error`
  - `verifyKey(key string) (bool, error)` — if a check row exists: true iff `DecryptWithKey(row) == constant`. If none: true iff no encrypted values exist, or the first encrypted value found in any of the four tables decrypts with `key`.
  - `hasCheckValue() (bool, error)`
  - `GetEncryptionReport() (*crypto.EncryptionReport, error)` — replaces the string-returning `GetEncryptionStatus()`; same counting.
  - `countEncryptedValues() (int, error)` — sum of `Encrypted` across the report.
  - `countPlaintextValues() (int, error)` — sum of `Plaintext`.

- [ ] **Step 1: Write the failing tests** (gocheck, `ModelsSuite`, each test `crypto.Unload()` at the end):
  - `TestCheckValueRoundTrip`: load key A; `writeCheckValue()`; `verifyKey(A)` true; `verifyKey(B)` false; `deleteCheckValue()`; `hasCheckValue()` false.
  - `TestVerifyKeyWithoutCheckValueUsesData`: load A, save an SMTP with password `"p"` via `PostSMTP` (stored encrypted), `Unload()`; `verifyKey(A)` true, `verifyKey(B)` false.
  - `TestVerifyKeyEmptyDatabase`: no check row, no encrypted values → `verifyKey(B)` true.
  - `TestEncryptionReportCounts`: one encrypted SMTP + one plaintext SMTP → `report.SMTPPasswords.Encrypted == 1`, `.Plaintext == 1`; `countEncryptedValues() == 1`.

- [ ] **Step 2: Run to verify failure**
  Run: `go test ./models/ -check.f "CheckValue|VerifyKey|EncryptionReport" -v`
  Expected: FAIL — undefined `writeCheckValue` etc.

- [ ] **Step 3: Implement** the migrations, `models/encryption_meta.go`, and the report change in `models/encryption.go`.

- [ ] **Step 4: Run to verify pass**
  Run: `go test ./models/ -v` (whole package — confirms the migration applies and nothing else broke)
  Expected: PASS.

- [ ] **Step 5: Commit**
  ```bash
  git add db/ models/
  git commit -m "feat: encryption check value table"
  ```

---

### Task 3: Encryption state machine

**Files:**
- Create: `models/encryption_state.go`
- Test: `models/encryption_state_test.go`

**Interfaces:**
- Consumes: Tasks 1–2.
- Produces:
  - Errors: `ErrEncryptionKeyUnavailable = errors.New("encryption key unavailable")`, `ErrEncryptionAlreadyOn`, `ErrEncryptedDataExists` (enable refused, use unlock), `ErrNotLocked`, `ErrWrongKey`, `ErrMigrationRunning`, `ErrNoKeyLoaded`, `ErrDecryptIncomplete`, `ErrBadConfirmation`.
  - `const DiscardConfirmationPhrase = "DISCARD ENCRYPTED DATA"`
  - `type EncryptionState string`: `"off"`, `"on"`, `"locked"`, `"disabling"`
  - `type EncryptionStatus struct { State EncryptionState \`json:"state"\`; Source crypto.KeySource \`json:"source"\`; LockedReason string \`json:"locked_reason,omitempty"\` /* "missing_key" | "wrong_key" | "invalid_key" */; StartupWarning string \`json:"startup_warning,omitempty"\`; Report *crypto.EncryptionReport \`json:"report"\`; PlaintextCount int \`json:"plaintext_count"\`; MigrationRunning bool \`json:"migration_running"\` }`
  - `InitEncryptionAtStartup() error` — rules below; returns error only for DB failures, never for key problems.
  - `GetEncryptionStatusInfo() (EncryptionStatus, error)`
  - `IsEncryptionLocked() bool`, `IsSessionKeyActive() bool` (cheap, no DB; used by templates on every page)
  - `EnableEncryptionWithKey(key string) error`
  - `UnlockEncryption(key string) error`
  - `RunEncryptionMigration(encrypt bool, dryRun bool) ([]string, error)`
  - `DisableEncryption() ([]string, error)`
  - `DiscardEncryptedData(confirmation string) (*crypto.EncryptionReport, error)` — returns the pre-discard counts

**Rules (from the spec):**
- Startup: env present & valid & `verifyKey` true → `LoadKey(env, SourceEnv)`; write a check value if none. Env present & valid & `verifyKey` false → Locked `wrong_key`, key **not** loaded. Env present & invalid format → Locked `invalid_key` if `countEncryptedValues() > 0`, else Off with `StartupWarning`; `log.Error` either way, no fatal. Env absent → Locked `missing_key` if encrypted values exist, else Off.
- Locked is a models-level flag (mutex-guarded) set only by startup and cleared by unlock or discard.
- Enable: refuse `ErrEncryptionAlreadyOn` if a key is loaded; `ErrEncryptedDataExists` if locked or encrypted values exist; else `LoadKey(key, SourceSession)` + `writeCheckValue()`. Does **not** migrate.
- Unlock: `ErrNotLocked` unless locked; `ErrWrongKey` unless `verifyKey`; then `LoadKey(key, SourceSession)`, write check value if missing, clear locked.
- Migrate: `ErrNoKeyLoaded` without a key; single-flight via a mutex `TryLock` → `ErrMigrationRunning`; dispatch to existing `MigrateToEncrypted`/`DryRunMigrateToEncrypted`/`MigrateToDecrypted`/`DryRunMigrateToDecrypted`.
- Disable: requires key; holds the migration lock; `crypto.SetDecryptOnly()`; `MigrateToDecrypted()`; if `countEncryptedValues() == 0` → `deleteCheckValue()`, `crypto.Unload()`; else return results + `ErrDecryptIncomplete` and stay decrypt-only (state `disabling`).
- Discard: `ErrNotLocked` unless locked; `ErrBadConfirmation` unless the phrase matches exactly; `UPDATE <table> SET <col> = '' WHERE <col> LIKE 'ENC:v1:%'` for `smtp.password`, `sms.provider_config`, `imap.password`, `events.details`; `deleteCheckValue()`; clear locked.
- Audit: every successful or failed action logs `log.Infof`/`log.Warnf` with the action and counts — never the key. The username is added by the API layer (Task 6).

- [ ] **Step 1: Write the failing tests.** Tests that need an env key use `os.Setenv(crypto.EncryptionKeyEnvVar, k)` + `defer os.Unsetenv(...)`; every test ends with `crypto.Unload()` and a reset of the locked flag (add unexported `resetEncryptionStateForTest()`).
  - `TestStartupNoEnvEmptyDB`: state `off`, source `none`.
  - `TestStartupEnvKeyFresh`: state `on`, source `environment`, `hasCheckValue()` true.
  - `TestStartupMissingKeyWithEncryptedData`: encrypt an SMTP with A, unload, unset env, startup → `locked`, reason `missing_key`.
  - `TestStartupWrongEnvKey`: data + check value under A; env = B → `locked`, `wrong_key`; `crypto.HasKey()` false.
  - `TestStartupMalformedEnvKey`: env = `"bad"`; with encrypted data → `locked`/`invalid_key`; on an empty DB → `off` with non-empty `StartupWarning`; startup returns nil both times.
  - `TestStartupLegacyNoCheckValue`: encrypted SMTP under A, delete the check row; env A → `on` and check value now present; env B → `locked`/`wrong_key`.
  - `TestEnableFromOff`: `EnableEncryptionWithKey(A)` → `on`, source `session`, `IsSessionKeyActive()` true; an existing plaintext SMTP is still plaintext (no auto-migrate); a newly posted SMTP is encrypted.
  - `TestEnableRefusedWhenEncryptedDataExists`: locked with data under A → `EnableEncryptionWithKey(B)` == `ErrEncryptedDataExists`; data unchanged.
  - `TestUnlock`: locked under A; `UnlockEncryption(B)` == `ErrWrongKey` and still locked; `UnlockEncryption(A)` → `on`, SMTP password decrypts to the original.
  - `TestMigrationSingleFlight`: hold the migration lock in the test, call `RunEncryptionMigration(true, false)` → `ErrMigrationRunning`.
  - `TestMigrateDryRunChangesNothing`: plaintext SMTP, key on, dry-run encrypt → report plaintext count unchanged.
  - `TestDisableWithConcurrentWrites`: key on, encrypted SMTP + events; run `DisableEncryption()` while a goroutine inserts 20 events with `Details`; afterwards `countEncryptedValues() == 0`, state `off`, `hasCheckValue()` false, all event details readable.
  - `TestDiscard`: locked with data; wrong phrase → `ErrBadConfirmation`, nothing changed; correct phrase → returned report has the prior counts, `countEncryptedValues() == 0`, state `off`.

- [ ] **Step 2: Run to verify failure**
  Run: `go test ./models/ -check.f "Startup|Enable|Unlock|Migrat|Disable|Discard" -v`
  Expected: FAIL — undefined `InitEncryptionAtStartup` etc.

- [ ] **Step 3: Implement `models/encryption_state.go`** per the rules above.

- [ ] **Step 4: Run to verify pass**
  Run: `go test -race ./models/ -v`
  Expected: PASS, no data race.

- [ ] **Step 5: Commit**
  ```bash
  git add models/
  git commit -m "feat: encryption state machine with lock, unlock, disable, discard"
  ```

---

### Task 4: Refuse encrypted credentials; flag locked event details

**Files:**
- Modify: `models/smtp.go:112` (`GetDialer`), `models/sms_request.go:112`, `models/smslog.go:158`, `models/campaign.go:95-137` (`Event`), `imap/imap.go:~68` (`Validate`), `imap/monitor.go:~410`
- Test: `models/encryption_lock_test.go`, `imap/` existing test file (add one case)

**Interfaces:**
- Consumes: `ErrEncryptionKeyUnavailable` (Task 3), `crypto.IsEncrypted`.
- Produces:
  - `RequirePlaintextCredential(value string) error` (exported, in `models/encryption_state.go`) — returns `ErrEncryptionKeyUnavailable` if `crypto.IsEncrypted(value)`.
  - `Event.DetailsLocked bool \`json:"details_locked,omitempty" gorm:"-"\``, set true in `AfterFind` when decryption fails (Details stays `""`).

- [ ] **Step 1: Write the failing tests:**
  - `TestSMTPGetDialerRefusesCiphertext`: SMTP with password set directly to `crypto.EncryptedPrefix+"x"` (no key loaded) → `GetDialer()` returns `ErrEncryptionKeyUnavailable`; with a plaintext password it returns a dialer.
  - `TestSMSRequestGetDialerRefusesCiphertext`: same for `SMSRequest.GetDialer` with `ProviderConfig` ciphertext.
  - `TestSMTPSaveWhileLockedKeepsCiphertext`: encrypt an SMTP under A, `Unload()`; `GetSMTP` (password is the raw `ENC:v1:` string), change only the name, `PutSMTP`; raw DB column equals the original ciphertext byte-for-byte; reload A → password decrypts to the original.
  - `TestEventDetailsLockedFlag`: insert an event with `Details` encrypted under A, `Unload()`, load it → `Details == ""`, `DetailsLocked == true`; a plaintext event → `DetailsLocked == false`, and the JSON has no `details_locked` key.
  - In `imap`: `Validate` with an `ENC:v1:` password returns `ErrEncryptionKeyUnavailable` without dialing.

- [ ] **Step 2: Run to verify failure**
  Run: `go test ./models/ ./imap/ -v`
  Expected: FAIL on the new cases.

- [ ] **Step 3: Implement:** call `RequirePlaintextCredential` at the top of each listed consumer (the password for SMTP/IMAP, `ProviderConfig` for SMS); `SMSLog.GetDialer` checks `campaign.SMS.ProviderConfig`; in `imap/monitor.go` skip that IMAP config with a `log.Warnf` (no dial) when refused. Set `DetailsLocked` in `Event.AfterFind`.

- [ ] **Step 4: Run to verify pass**
  Run: `go test ./models/ ./imap/ ./mailer/ -v`
  Expected: PASS (existing tests, including `campaign_test.go:720-741`, stay green).

- [ ] **Step 5: Commit**
  ```bash
  git add models/ imap/
  git commit -m "feat: refuse encrypted credentials when key unavailable"
  ```

---

### Task 5: Startup wiring and CLI flag removal

**Files:**
- Modify: `gophish.go` (lines ~48-160 handlers, 170-174 flags, 195-252 startup)

**Interfaces:**
- Consumes: `models.InitEncryptionAtStartup()`.

- [ ] **Step 1:** Delete `handleGenerateEncryptionKey`, `handleEncryptionStatus`, `handleMigrateEncryption`, `handleMigrateDecryption`, the five flags, and their dispatch blocks. Replace the `crypto.InitEncryption()` block after `models.Setup(conf)` with `models.InitEncryptionAtStartup()` (fatal only on its returned DB error) and log the resulting state at Info (`"Database encryption: on (environment)"`, `"... LOCKED (missing_key) — unlock from Settings > Encryption"`, etc.). Remove the `crypto` import if unused, and any Task 1 shim.

- [ ] **Step 2: Verify the build and the flags are gone**
  Run: `go build -o anglerphish-test.exe . && ./anglerphish-test.exe --help`
  Expected: builds; help output contains none of `generate-encryption-key`, `migrate-encryption`, `migrate-decryption`, `encryption-status`, `dry-run`. Delete `anglerphish-test.exe` afterwards.

- [ ] **Step 3: Run the full suite**
  Run: `go test ./...`
  Expected: PASS.

- [ ] **Step 4: Commit**
  ```bash
  git add gophish.go
  git commit -m "refactor: replace encryption CLI flags with startup state init"
  ```

---

### Task 6: Admin encryption API

**Files:**
- Create: `controllers/api/encryption.go`, `controllers/api/encryption_test.go`
- Modify: `controllers/api/server.go` (routes next to the webhooks block, lines ~113-117)

**Interfaces:**
- Consumes: Task 3 functions and errors.
- Produces (all `mid.Use(handler, mid.RequirePermission(models.PermissionModifySystem))`, JSON):
  - `GET  /api/encryption/status` → `models.EncryptionStatus`
  - `POST /api/encryption/generate` → `{"key": "<44 chars>"}`, header `Cache-Control: no-store`; does not load the key
  - `POST /api/encryption/enable` body `{"key"}` → status
  - `POST /api/encryption/unlock` body `{"key"}` → status
  - `POST /api/encryption/migrate` body `{"direction": "encrypt"|"decrypt", "dry_run": bool}` → `{"results": [...], "status": ...}`
  - `POST /api/encryption/disable` → `{"results": [...], "status": ...}`
  - `POST /api/encryption/discard` body `{"confirmation"}` → `{"discarded": <report>, "status": ...}`
  - Error mapping: `ErrWrongKey`, `ErrBadConfirmation`, `crypto.ErrInvalidKeyLength`, bad direction → 400; `ErrMigrationRunning`, `ErrEncryptionAlreadyOn`, `ErrEncryptedDataExists`, `ErrNotLocked`, `ErrNoKeyLoaded` → 409; `ErrDecryptIncomplete` → 409 carrying the results; other → 500 with a generic message. Error bodies never include request input.
  - Unlock rate limit: 5 failed attempts per user per 15 minutes → 429 (in-memory map keyed by user id, mutex-guarded). Designer's call on the values.
  - Audit: each handler logs `log.Infof("encryption: <action> by %s ...", user.Username, counts)`.
  - The paste-back check (does the pasted key equal the generated one) is done client-side in Task 8; `enable` validates format only.

- [ ] **Step 1: Write the failing tests** (`setupTest(t)` + `createUnpriviledgedUser(t, models.RoleUser)` patterns from `user_test.go`; reset crypto/state in `t.Cleanup`):
  - `TestEncryptionEndpointsForbiddenForNonAdmin`: table over all 7 routes with the unprivileged user's Bearer key → 403.
  - `TestGenerateReturnsKeyNoStore`: 200; `len(key) == 44`; `Cache-Control` == `no-store`; status afterwards still `off`.
  - `TestEnableThenStatus`: enable with a generated key → `state == "on"`, `source == "session"`; the status JSON body does not contain the key string.
  - `TestEnableInvalidKey`: `{"key":"abc"}` → 400; body does not contain `abc`.
  - `TestUnlockWrongKeyAndRateLimit`: put the state machine into locked; 5 wrong unlocks → 400 each, the 6th → 429; body never contains the submitted key.
  - `TestMigrateBadDirection`: `{"direction":"sideways"}` → 400.
  - `TestDiscardWrongPhrase`: locked; `{"confirmation":"discard"}` → 400; counts unchanged.
  - `TestKeyNeverLogged`: redirect the `logger` output to a buffer for the test (or attach a logrus hook), run generate → enable → disable; assert the buffer does not contain the key.

- [ ] **Step 2: Run to verify failure**
  Run: `go test ./controllers/api/ -run Encryption -v`
  Expected: FAIL — 404s / undefined handlers.

- [ ] **Step 3: Implement** the handlers and routes.

- [ ] **Step 4: Run to verify pass**
  Run: `go test -race ./controllers/api/ -v`
  Expected: PASS.

- [ ] **Step 5: Commit**
  ```bash
  git add controllers/api/
  git commit -m "feat: admin encryption API"
  ```

---

### Task 7: Banners in the base layout

**Files:**
- Modify: `controllers/route.go:200-226` (`templateParams`, `newTemplateParams`), `templates/base.html` (directly after the navbar, before page content)
- Test: `controllers/route_test.go`

**Interfaces:**
- Consumes: `models.IsEncryptionLocked()`, `models.IsSessionKeyActive()`.
- Produces: `templateParams.EncryptionLocked bool`, `templateParams.EncryptionSessionKey bool`.

**Copy (exact):**
- Admin + locked (red, `alert-danger`): `Encrypted data found but no valid encryption key is loaded. Sending with encrypted credentials is blocked.` + link `Unlock in Settings` → `/settings#encryptionSettings`.
- Non-admin + locked (`alert-warning`): `Some sending is unavailable — contact an administrator.`
- Admin + session key (`alert-warning`): `Encryption key is active for this session only. Add it to your environment or secret manager before the next restart.`

- [ ] **Step 1: Write the failing tests** in `controllers/route_test.go` following its existing login/request helpers: log in as admin with the locked flag set via `models.SetEncryptionLockedForTest(locked bool)` (add it in `models/encryption_state.go`, doc-commented as test-only; it sets the locked flag with reason `missing_key` and is also used by Task 6 tests to reach the locked state) → `GET /` body contains `Unlock in Settings`; as a `RoleUser` → contains `contact an administrator` and not `Unlock in Settings`; with no lock → neither string.

- [ ] **Step 2: Run to verify failure**
  Run: `go test ./controllers/ -run Banner -v`
  Expected: FAIL.

- [ ] **Step 3: Implement** the two fields and the banner markup.

- [ ] **Step 4: Run to verify pass**
  Run: `go test ./controllers/ -v`
  Expected: PASS.

- [ ] **Step 5: Commit**
  ```bash
  git add controllers/route.go controllers/route_test.go templates/base.html models/
  git commit -m "feat: encryption lock and session key banners"
  ```

---

### Task 8: Settings > Encryption tab

**Files:**
- Modify: `templates/settings.html` (new `<li>` + `tab-pane id="encryptionSettings"`, both inside `{{if .ModifySystem}}`), `static/js/src/app/settings.js`
- Build: `static/js/dist/app/settings.min.js` (via the project's gulp/webpack build)

**Interfaces:**
- Consumes: Task 6 endpoints (send the CSRF/API key the same way the existing settings.js calls do).

**Behaviour per state** (from `GET /api/encryption/status`):
- `off`: explanation + *Generate key*. Generate → modal showing the key (read-only input + copy button + env/secret-manager instructions) and a second input *Paste the key to confirm you saved it*; *Turn on encryption* stays disabled until the pasted value `===` the generated key; then `POST enable`; afterwards drop the key from JS memory and the DOM.
- `on`: source badge (`environment` / `session only`), per-field table (encrypted / plaintext / empty / total), *Preview encryption* / *Encrypt existing data*, *Preview decryption* / *Disable encryption* (confirm modal showing the preview). Show a prompt to run *Encrypt existing data* when `plaintext_count > 0`. After disable succeeds, show: `Encryption is off. Remove ANGLERPHISH_ENCRYPTION_KEY from your environment or secret manager — if it is still set at the next restart, encryption will turn back on with that key.`
- `locked`: reason text (`missing_key` / `wrong_key` / `invalid_key`), plaintext count note, *Unlock* (password-type input + button), and a collapsed danger section *Discard encrypted data* listing the counts to be lost and requiring the exact phrase `DISCARD ENCRYPTED DATA`.
- `disabling`: results of the incomplete decrypt and *Retry disable*.
- Buttons disable while `migration_running`; 409/429 messages are shown as flashes.

- [ ] **Step 1: Implement** the markup and JS.
- [ ] **Step 2: Build assets**
  Run: `npx gulp scripts`
  Expected: `static/js/dist/app/settings.min.js` regenerated without errors.
- [ ] **Step 3: Manual verification in the running app** (use the `run` skill). Start with no env key and the throwaway DB `-config` pointing at a copy of `gophish.db`. Walk through: generate → paste mismatch keeps the button disabled → paste match → on/session + banner on another page → encrypt existing data → restart without env → locked banner, SMTP send test fails with `encryption key unavailable` → unlock wrong key error → unlock right key → disable → off. Separately: locked → discard with the phrase → off. Log in as a `RoleUser`: no Encryption tab, the generic banner only while locked. Check the browser network tab: the key appears only in the generate response, with `Cache-Control: no-store`.
- [ ] **Step 4: Commit**
  ```bash
  git add templates/settings.html static/js/src/app/settings.js static/js/dist/app/settings.min.js
  git commit -m "feat: encryption settings tab"
  ```

---

### Task 9: Documentation and final verification

**Files:**
- Modify: `templates/api_documentation.html` (~1099-1216), `FEATURES.md:71`, `CHANGELOG.md`

- [ ] **Step 1:** Rewrite the encryption section of `api_documentation.html`: remove all CLI flag docs; describe the Settings > Encryption flow (generate + paste-back, encrypt existing data, locked/unlock, disable, discard); list the new `/api/encryption/*` endpoints; give env injection examples for systemd `EnvironmentFile`/`LoadCredential`, Docker secrets, and Vault Agent. Update `FEATURES.md:71` (replace "CLI commands" with the UI) and add a `CHANGELOG.md` entry matching existing entry style (note: CLI flags removed).
- [ ] **Step 2: Verify no stale references**
  Run: `git grep -nE "generate-encryption-key|migrate-encryption|migrate-decryption|encryption-status|--dry-run" -- . ':!docs/superpowers'`
  Expected: no output.
- [ ] **Step 3: Full suite with race detector**
  Run: `go test -race ./...`
  Expected: PASS.
- [ ] **Step 4: Commit**
  ```bash
  git add templates/api_documentation.html FEATURES.md CHANGELOG.md
  git commit -m "docs: UI encryption management"
  ```
