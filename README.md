# Tanabata File Manager

A multi-user, tag-based web file manager for images and video. Go + Gin backend
(Clean Architecture, pgx, goose migrations), SvelteKit SPA frontend, PostgreSQL,
JWT auth — shipped as a single Docker image that serves both the API and the
built SPA on one port.

## Documentation

- [`openapi.yaml`](openapi.yaml) — full REST API specification
- [`docs/DEPLOY.md`](docs/DEPLOY.md) — production deploy (Gitea Actions → host)
- [`docs/GO_PROJECT_STRUCTURE.md`](docs/GO_PROJECT_STRUCTURE.md) — backend architecture
- [`docs/FRONTEND_STRUCTURE.md`](docs/FRONTEND_STRUCTURE.md) — frontend architecture
- [`.env.example`](.env.example) — every configuration variable, documented

## Quick start

```bash
cp .env.example .env        # then edit the secrets (JWT_SECRET, ADMIN_PASSWORD, …)
docker compose up -d --build
```

By default this runs the app plus a bundled PostgreSQL container
(`COMPOSE_PROFILES=with-db`). To point at a Postgres already on the host, set
`COMPOSE_PROFILES=` empty and aim `DATABASE_URL` at `host.docker.internal`. See
[`.env.example`](.env.example) for the full matrix.

The app is published on **127.0.0.1** only and expects a reverse proxy in front
(see below). The default port is **42776** — the sum of the Unicode code points
of 七夕.

## Reverse proxy (nginx)

The container publishes its port on loopback (`127.0.0.1:${APP_PORT}:42776` in
[`docker-compose.yml`](docker-compose.yml)), so a reverse proxy on the host
terminates TLS and forwards to it. Three settings matter for this app:

1. **`client_max_body_size`** — uploads go up to `MAX_UPLOAD_BYTES` (500 MiB by
   default). nginx caps request bodies at **1 MiB** out of the box, so without
   this every large upload fails with `413`.
2. **Forwarded headers** — the app trusts `X-Forwarded-For` only from the hops in
   `TRUSTED_PROXIES` (default: loopback + Docker bridge ranges) and keys its
   login/refresh rate limiter on the resulting client IP. If the proxy doesn't
   send the header, every request looks like it comes from the proxy and shares
   one rate-limit bucket.
3. **Streaming for big media** — turning request/response buffering off lets
   large uploads stream straight to the app and lets video range-seeks work
   without nginx spooling whole files to disk first.

```nginx
server {
    listen 443 ssl;
    server_name tanabata.example.com;

    # ssl_certificate / ssl_certificate_key ... (e.g. from certbot)

    # Match MAX_UPLOAD_BYTES (500 MiB default); nginx defaults to 1m → 413.
    client_max_body_size 512m;

    location / {
        proxy_pass http://127.0.0.1:42776;   # APP_PORT
        proxy_http_version 1.1;

        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Stream large uploads/downloads instead of buffering to disk; keeps
        # video range-seek responsive. Scope these to file/preview locations
        # instead if you'd rather keep buffering for small JSON responses.
        proxy_request_buffering off;
        proxy_buffering         off;
        proxy_read_timeout      300s;
        proxy_send_timeout      300s;
    }
}
```

If you run the app **without** a proxy and want it reachable on the LAN, drop the
`127.0.0.1:` prefix from the `ports` line in
[`docker-compose.yml`](docker-compose.yml) and adjust `TRUSTED_PROXIES`
accordingly.

## Development

```bash
# Backend
cd backend
go run ./cmd/server          # dev server
go test ./...                # all tests

# Frontend
cd frontend
npm run dev                  # Vite dev server
npm run build                # production build
npm run generate:types       # regenerate API types from openapi.yaml
```
