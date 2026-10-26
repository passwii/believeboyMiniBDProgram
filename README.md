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

## Checks

```bash
make test
make check
```

The system design is documented in
`docs/superpowers/specs/2026-10-26-believeboy-mini-program-design.md`.
