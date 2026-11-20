# FutureVoice

FutureVoice is a small school-feedback system built as one Go codebase with two separate processes and a separate React admin application:

- `cmd/server` — Gin HTTP API, business logic wiring, PostgreSQL, sessions and metrics.
- `telegram` — Telego bot. It receives Telegram updates and talks to the backend over HTTP; it does not access PostgreSQL.
- `web` — React/Vite admin and moderator panel. It is intentionally written in JSX/JavaScript (not TypeScript) to keep the frontend approachable; it talks to the same backend API and contains no server secrets.
- `ops` — optional Prometheus + Grafana monitoring stack.

The backend is the source of truth for users, roles, requests, moderation actions and analytics. Telegram and React are clients; neither client is trusted to enforce permissions.

## Requirements

- Go 1.26.7 or newer compatible with the `go` directive in `go.mod`.
- PostgreSQL 14 or newer (newer supported versions are fine).
- Node.js 22 LTS and npm for the React panel.
- Docker Compose only if you want to run Prometheus and Grafana.
- A Telegram bot created with [@BotFather](https://t.me/BotFather).

The original archive did not contain `go.mod` or `go.sum`. `go.mod` has been restored. Generate `go.sum` on your machine after restoring network access:

```powershell
go mod tidy
```

Commit the generated `go.sum` together with `go.mod`; do not commit `.env` or `web/.env`.

## Configuration

Copy the root example:

```powershell
Copy-Item .env.example .env
```

Configure the following values in `.env`:

- `BOT_TOKEN` — Telegram bot token from BotFather. Only the Telegram process needs it.
- `BOT_API_SECRET` — long random secret shared by Telegram and backend. It protects only internal bot-to-backend endpoints.
- `BACKEND_URL` — backend address as seen by the Telegram process. The default `http://127.0.0.1:8080` is for both processes running on the same host.
- `PORT` — backend listen port; defaults to `8080`.
- `DATABASE_URL` — PostgreSQL connection string.
- `TELEGRAM_OIDC_CLIENT_ID`, `TELEGRAM_OIDC_CLIENT_SECRET`, `TELEGRAM_OIDC_REDIRECT_URL` — credentials for the separate web panel login. All three must be configured together.
- `ADMIN_WEB_URL` — browser origin to which the OIDC callback returns, normally `http://localhost:5173` in development.
- `COOKIE_SECURE` — `false` only for local HTTP development; set to `true` behind HTTPS in production. The backend refuses a non-local `ADMIN_WEB_URL` unless HTTPS and Secure cookies are enabled.
- `ADMIN_SESSION_TTL` — server-side admin session lifetime, default `8h`.
- `METRICS_USER` and `METRICS_PASSWORD` — both are required to expose the protected Prometheus endpoint.
- `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD` — Grafana login; choose a unique password.

Generate random secrets in PowerShell:

```powershell
[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
```

Use different random values for `BOT_API_SECRET`, `METRICS_PASSWORD`, and `GRAFANA_ADMIN_PASSWORD`. Never put these values in React/Vite variables.

### Telegram web login (OIDC)

The admin panel uses Telegram's OpenID Connect login, not the bot token and not a user-supplied Telegram ID.

1. Open the @BotFather mini app, select your bot, then open **Login Widget**. Register the exact website origin and redirect URL as Allowed URLs. Telegram's current OIDC instructions use this screen to issue the Client ID and Client Secret.
2. Use `http://localhost:5173` as the local website origin.
3. Register this exact callback URL: `http://localhost:5173/api/v1/auth/telegram/callback`.
4. Copy the OIDC Client ID and Client Secret issued for the login integration into the three `TELEGRAM_OIDC_*` variables (Client ID, Client Secret and redirect URL).
5. Start the bot and send `/start` from your own Telegram account so that your user record exists in PostgreSQL.
6. Promote that existing account to administrator using its Telegram ID:

```sql
UPDATE users
SET role = 'admin'
WHERE telegram_id = 123456789;
```

7. Open the web panel and choose **Войти через Telegram**. Only an existing, active `admin` or `moderator` account is allowed into the panel. OIDC login does not create an account and does not grant a role.

The backend validates the OIDC ID token, issuer, audience and nonce; the flow also uses `state` and PKCE. The browser receives an opaque HttpOnly session cookie. Mutating requests require the CSRF token header. The Telegram bot's `BOT_API_SECRET` is never sent to the browser.

If Telegram OIDC discovery is temporarily unavailable when the backend starts, the request API remains available but new web logins are disabled until the backend is restarted after connectivity is restored. Existing server sessions can still be validated.

## Run locally on Windows

### 1. Start PostgreSQL

Create a database and user, then set `DATABASE_URL` in `.env`. The backend runs the schema initialization when it starts. Back up an existing database before upgrading because the schema initializer includes compatibility changes for earlier request tables.

### 2. Start the backend

In the first PowerShell window, from the repository root:

```powershell
go mod tidy
go run ./cmd/server
```

The backend listens on `http://127.0.0.1:8080` by default. `GET /healthz` is a simple availability check.

### 3. Start the Telegram bot

In the second PowerShell window:

```powershell
go run ./telegram
```

Send `/start` to the bot. The bot registers or refreshes the user's Telegram profile through the internal backend API. It does not write to the database directly.

### 4. Start the React admin panel

Copy the frontend environment example:

```powershell
Copy-Item web/.env.example web/.env
```

Then:

```powershell
cd web
npm ci
npm run dev
```

Open `http://localhost:5173`. Vite proxies `/api/*` to the Go backend. If your backend is on another host, set `API_PROXY_TARGET` in `web/.env` to its URL; use HTTPS for a remote backend.

For a production build:

```powershell
npm run build
```

The static files are written to `web/dist`. Serve them with Nginx or another static-file server, and proxy `/api/` to the Go backend on the same origin. Use the public website origin for `ADMIN_WEB_URL` and the exact public callback URL for `TELEGRAM_OIDC_REDIRECT_URL`. Set `COOKIE_SECURE=true` in production.

## Admin panel

The panel has three main areas:

- **Обзор** — business analytics. Visible to administrators only.
- **Заявки** — paginated moderation queue with status/type/search filters, request details, status changes, assignment to the current moderator, two-way student messaging, internal notes, event history and administrator-only soft deletion.
- **Пользователи** — administrator-only role and account activation management.

Moderators can authenticate, view the moderation queue, assign requests to themselves, update status, reply to students in Telegram and write private internal notes. Administrators can additionally see analytics, manage user roles/access, and delete requests from the panel. These checks are performed in the backend; hiding a control in React is not authorization.

The request panel API is under `/api/v1/admin`. It uses the authenticated session's user ID; it does not accept an arbitrary acting user ID from the browser.

## Two kinds of analytics

### 1. Business analytics inside FutureVoice

The **Обзор** page requests aggregated values from:

```text
GET /api/v1/admin/analytics/overview?from=YYYY-MM-DD&to=YYYY-MM-DD
```

It displays:

- number of created requests;
- current counts by status for requests created in the selected period;
- resolution percentage;
- average time to the first final decision (completion or rejection);
- daily request creation and final-decision dynamics (completion or rejection);
- distribution by request type and status.

The period can cover up to 366 inclusive calendar days. Each analytics card has its own PNG export (summary, dynamics, statuses, request types and processing efficiency); the main button exports all sections in one image. Exports use the selected period and draw an infographic in the browser from the aggregate response. The export intentionally excludes names, Telegram IDs, request descriptions and individual request IDs so that it can be used for school-channel posts without disclosing personal information.

These are business metrics calculated from PostgreSQL. They are not Prometheus time-series metrics.

### 2. Operational monitoring: Prometheus + Grafana

This stack monitors the health and performance of the Go backend:

- HTTP request rate and 5xx percentage;
- HTTP latency p95;
- in-flight requests;
- Go goroutines and process/runtime metrics;
- PostgreSQL connection-pool usage and waits.

The backend exposes `/metrics` only when both `METRICS_USER` and `METRICS_PASSWORD` are configured. The endpoint uses HTTP Basic Authentication. Do not expose it publicly without network restrictions and HTTPS. Compose writes these credentials into a private Docker volume through a one-shot init container; Prometheus reads them with `username_file` and `password_file`, so the password is not embedded in the Prometheus YAML.

Start monitoring from the repository root:

```powershell
docker compose --env-file .env -f ops/compose.monitoring.yml up -d
```

Open:

- Grafana: `http://localhost:3000` (credentials from `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD`).
- Prometheus: `http://localhost:9090` (local diagnostic UI; in the supplied Compose file it is bound to loopback).

The dashboard is provisioned automatically from `ops/grafana/dashboards/futurevoice-operations.json`. Prometheus scrapes the Go HTTP backend at `host.docker.internal:8080`; the Telegram polling process is not yet instrumented with Prometheus metrics and continues to report through application logs. If you change `PORT`, update the target in `ops/prometheus/prometheus.yml` too.

Stop the stack:

```powershell
docker compose --env-file .env -f ops/compose.monitoring.yml down
```

Prometheus and Grafana data are kept in Docker volumes. To delete the monitoring history permanently, remove the volumes explicitly after confirming you no longer need them.

**Privacy:** Prometheus labels contain only low-cardinality technical values such as method, route template and status code. Do not add Telegram IDs, student names, request IDs or request text as metric labels. Those belong in protected application data, not monitoring labels.

## Request workflow and access

- A student can create a request, view its current status and open their own request list through Telegram.
- Status changes (`open`, `in_progress`, `waiting_for_user`, `closed`, `rejected`) create a Telegram notification. Rejection requires a reason; completion can include a result.
- On backend startup, the schema initializer updates the allowed status constraint and creates the Telegram notification outbox table. Make a PostgreSQL backup before the first restart with this version.
- A student can open each request using inline buttons below the notification or in the “Мои заявки” list. The request card shows the public conversation and available actions. Replies are sent through the inline “Написать команде” button; rejected requests offer “Подать апелляцию”. No request IDs or chat commands need to be typed. The student can soft-delete only their own request while it is `open`.
- Active moderators and administrators can see the moderation queue, assign requests, update status, send public replies to Telegram and add internal notes that remain hidden from students.
- Only administrators can manage users and delete requests from the admin panel.
- Roles are read from PostgreSQL for every protected operation. Telegram UI and React UI do not decide permissions.
- Request status changes, assignments, public replies, internal notes and soft-deletion are recorded in `request_events`/`request_comments`.
- Telegram notifications are written to a PostgreSQL outbox in the same transaction as the status/message change. The bot worker delivers them and retries temporary failures. Delivery is at-least-once: a very rare duplicate is possible if Telegram accepted a message but the bot stopped before acknowledging it.
- Public conversation messages are visible to the student; internal notes are not exposed to students. Both are shown in the protected admin request details page. The page refreshes the conversation and event history every five seconds.

## Context, concurrency and shutdown

Telegram handlers pass their handler context into the backend HTTP client. Gin handlers pass `c.Request.Context()` into use cases and PostgreSQL methods. The backend uses graceful shutdown with a bounded timeout. In-memory Telegram form drafts expire after 30 minutes and are lost on bot restart; already-created requests are persisted in PostgreSQL.

## Checks

Run Go tests and vet using the Go version in `go.mod`:

```powershell
go test ./...
go vet ./...
```

Run frontend lint and build:

```powershell
cd web
npm run lint
npm run build
```

Before production, also test OIDC login/logout, CSRF rejection, the role matrix (student/moderator/admin), request updates, database migrations on a restored copy of production data, and recovery from a database backup.
