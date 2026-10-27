# BelieveBoy Viewer

Responsive H5 viewer for PIN-protected proposal shares.

```bash
cp .env.example .env.local
pnpm dev
```

Development URL: `http://view.localhost:5173/s/<share-token>`

The viewer proxies `/api` to `API_ORIGIN` and loads signed proposal HTML from the
separate content origin in a sandboxed iframe.
