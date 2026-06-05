# Vortex — Security Checklist

Status: **local-only dev** (no git remote, services on localhost, single user).
The items below are **deferred on purpose** — none are exploitable while the app
runs only on one machine. Each is tagged with the **trigger** that makes it
mandatory. Do not let any of these slip past its trigger.

Legend:
- 🚪 **before-public-remote** — do before pushing to GitHub / any remote.
- 🚀 **before-deploy** — do before the server is reachable off your PC.
- 👥 **before-multi-user** — do before anyone else gets an account.

---

## Must-fix before the code goes public (🚪)
- [ ] **Remove all real/default secrets from the repo.** Move `JWT_SECRET`,
      `CENTRIFUGO_SECRET`, `CENTRIFUGO_API_KEY`, DB/Redis/MinIO creds to `.env`
      (gitignored). Commit only `.env.example` with placeholder values.
      Files today: `server/pkg/config/config.go`, `deploy/centrifugo/config.json`,
      `docker-compose.yml`.
- [ ] **Rotate every secret** that was ever committed (they live in git history).

## Must-fix before deploy (🚀)
- [ ] **Config fail-fast:** server refuses to start in prod if any secret is
      still the dev default. (Partial: scaffolded in config — keep enforcing.)
- [ ] **Private media:** stop making the MinIO bucket world-readable
      (`media/storage.go ensureBucket`). Serve attachments via short-lived
      presigned URLs or an authenticated proxy that checks conversation membership.
- [ ] **Lock down Centrifugo admin** (`admin:false` or strong password, not on a
      public port) and **MinIO console** creds.
- [ ] **CORS allowlist** instead of `*` (`config.go` `CORS_ALLOWED_ORIGINS`).
- [ ] **TLS** in front of the API and WebSocket.
- [ ] **Rate limiting** on auth + message-send (Redis-backed).

## Must-fix before multi-user (👥)
- [ ] **JWT/session revocation:** validate the session in claims still exists
      (logout / logout-all should kill access tokens, not just refresh).
- [ ] **Fix account enumeration** on register (generic conflict, don't reveal
      username-vs-email).
- [ ] **Channel permission enforcement** (roles exist in schema, unused in code).
- [ ] Trustworthy client IP (don't blindly trust `X-Forwarded-For`).
- [ ] **Read receipts are self-reported:** `POST /conversations/{id}/read` is
      driven by the client, so a user could under-report having read a message.
      Fine for MVP; no real anti-spoofing until it matters.
- [ ] **Typing/read events** have no per-user rate limit (publish is membership-
      gated but a member could spam). Fold into the general rate-limiting work.

## Build-it-right (apply continuously, no trigger)
- [x] Parameterized SQL everywhere (already good).
- [ ] Input length caps on every user-supplied field (message content, etc.).
- [ ] Membership/authorization check on every read & write path.
- [ ] Media kept private-by-default; never trust client-supplied filenames/types.
- [ ] Validate `username` charset; cap password length (bcrypt 72-byte limit).
