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
