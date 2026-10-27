# BelieveBoy Admin

Next.js admin console for uploading self-contained HTML proposals and managing
public or private share links.

```bash
cp .env.example .env.local
pnpm dev
```

Development URL: `http://admin.localhost:3000`

The `/api` route proxies to `API_ORIGIN` so administrator cookies remain on the
admin origin.
