# BelieveBoy Mini Program

[![CI](https://github.com/passwii/believeboyMiniBDProgram/actions/workflows/ci.yml/badge.svg)](https://github.com/passwii/believeboyMiniBDProgram/actions/workflows/ci.yml)

Private HTML proposal delivery system with a WeChat mini program, responsive H5
viewer, Next.js admin console, and Go API.

## Local setup

```bash
cp .env.example .env
make up
make migrate
cd services/api
go run ./cmd/admin create --email admin@example.com
go run ./cmd/server
```

The API listens on `http://localhost:8080` by default.

The administrator command prompts for a credential without echoing it. For
non-interactive setup, pipe the credential and pass `--credential-stdin`.

Administrator security operations are rate-limited to five failed attempts per
IP in ten minutes, followed by a fifteen-minute cooldown. Audit events are
available to administrators at `GET /api/v1/admin/audit-events`; IP and
User-Agent values are stored as keyed hashes. The API removes expired sessions
and tickets hourly, and retains audit events for 180 days.

Client IPs are taken from the TCP peer by default. When running behind a reverse
proxy, set `TRUSTED_PROXY_CIDRS` to the proxy CIDR ranges; only then will the API
use `X-Forwarded-For` from those peers for rate limits and audit hashes.

The first-release admin login limiter is process-local and assumes one API
instance. Use a shared limiter store before scaling the API horizontally.

## Frontend development

```bash
pnpm --dir apps/admin dev
pnpm --dir apps/viewer dev
```

- Admin console: `http://admin.localhost:3000`
- H5 viewer: `http://view.localhost:5173`
- WeChat mini program: open `apps/miniapp` in WeChat Developer Tools

The admin console proxies `/api` to `API_ORIGIN` (`http://localhost:8080` by
default). The viewer does the same through the Vite dev proxy.

Before building the mini program for a real environment, update
`apps/miniapp/miniprogram/utils/config.ts` with the HTTPS API and viewer
origins, and add both origins to the WeChat domain allowlist.

## Checks

```bash
make test
make check
```

The system design is documented in
`docs/superpowers/specs/2026-10-26-believeboy-mini-program-design.md`.
