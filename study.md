# 📚 Notifly — Codebase Study Guide

> **Purpose:** This document is the single source of truth for understanding the Notifly codebase.
> Read this **before** writing any code. Keep it updated when the architecture evolves.

---

## 1. What Is Notifly?

Notifly is a **standalone notification microservice** built in Go.
It receives requests from other backend services (e.g., jarz) and delivers notifications through pluggable channels — currently **email via Resend**, with SMS and push planned for the future.

### Why It Exists

- **Decouples notification logic** from application backends so any app can send branded emails through one API.
- **Template-driven**: all email content lives in HTML templates, not hardcoded in code.
- **Multi-channel by design**: adding SMS (Twilio) or push (FCM) requires only a new `Provider` implementation — zero changes to domain logic.
- **Async processing**: notifications are queued and processed in the background, so callers never block on provider latency.
- **Delivery tracking**: every notification is logged with full lifecycle tracking (queued → sent → delivered → opened/bounced).
- **Idempotent**: duplicate requests with the same `idempotency_key` are safely deduplicated.
- **Rate limited per recipient**: prevents accidentally spamming users (configurable max per hour).
- **Self-healing**: a stale task reaper automatically recovers orphaned notifications — no task is ever permanently lost.

### What It Does NOT Handle

- User management (that's the calling app's job)
- Deciding *when* to notify (that's business logic in the caller)
- Authentication of end users (only authenticates calling services via API keys)

---

## 2. Tech Stack

| Layer                       | Technology                                              |
| --------------------------- | ------------------------------------------------------- |
| Language                    | Go 1.25                                                 |
| HTTP Framework              | Gin (`github.com/gin-gonic/gin`)                        |
| Config                      | Viper + godotenv (`NOTIFLY_` prefix)                    |
| Email Provider              | Resend REST API                                         |
| Templating                  | Go `html/template`                                      |
| Task Queue                  | Asynq + Redis (`github.com/hibiken/asynq`)              |
| Persistence                 | Supabase (`github.com/supabase-community/supabase-go`)  |
| Rate Limiting (IP)          | `golang.org/x/time/rate` (token bucket)                 |
| Rate Limiting (Recipient)   | Redis sorted sets (sliding window)                      |
| CORS                        | `github.com/gin-contrib/cors`                           |
| Containerization            | Docker + Docker Compose (multi-stage Alpine build)      |
| Logging                     | `log/slog` (structured JSON)                            |

---

## 3. Project Structure

```
notifly/
├── cmd/
│   ├── server/
│   │   └── main.go                  # HTTP API entry point — wiring, server, graceful shutdown
│   └── worker/
│       └── main.go                  # Queue worker + reaper entry point — asynq server, task processing
├── internal/
│   ├── config/
│   │   └── config.go                # Viper-based config loader (Redis, Supabase, queue, reaper)
│   ├── common/
│   │   ├── errors.go                # Domain error types (Validation, NotFound, Provider, Unauthorized)
│   │   └── response.go              # Standardized API response envelope & error mapper
│   ├── domain/
│   │   └── notification/
│   │       ├── model.go             # Request/response DTOs, Channel & NotificationType enums
│   │       ├── log_model.go         # NotificationLog model, ListFilter, ListResponse
│   │       ├── provider.go          # Provider & TemplateRenderer interfaces (ports)
│   │       ├── store.go             # NotificationStore interface (port) — includes ListStale
│   │       ├── ratelimit.go         # RecipientRateLimiter interface (port)
│   │       ├── task.go              # Asynq task type & payload serialization
│   │       ├── service.go           # Business logic: validate → idempotency → rate limit → enqueue
│   │       ├── worker.go            # Queue worker: fetch log → render → send → update status
│   │       ├── reaper.go            # Stale task reaper: periodic DB reconciliation loop
│   │       └── handler.go           # HTTP handlers — send, list, get, webhooks
│   ├── infra/
│   │   ├── email/
│   │   │   └── resend.go            # Resend API implementation of Provider interface
│   │   ├── template/
│   │   │   ├── engine.go            # Template engine implementing TemplateRenderer
│   │   │   └── templates/           # 11 HTML email templates
│   │   ├── store/
│   │   │   └── supabase.go          # Supabase SDK implementation of NotificationStore
│   │   ├── queue/
│   │   │   └── asynq.go             # Asynq client/server wrappers, enqueue helper
│   │   └── ratelimit/
│   │       └── recipient.go         # Redis sliding-window per-recipient rate limiter
│   ├── middleware/
│   │   ├── auth.go                  # X-API-Key header validation (constant-time compare)
│   │   ├── cors.go                  # CORS policy from config
│   │   ├── ratelimit.go             # Per-IP token bucket rate limiter
│   │   └── requestid.go             # X-Request-ID injection (UUID v4)
│   └── router/
│       └── router.go                # Gin engine assembly — middleware stack & route registration
├── migrations/
│   └── 001_init.sql                  # Full DB schema + indexes (run in Supabase SQL Editor)
├── config.yaml                       # Default config (overridable by env vars)
├── .env / .env.example               # Environment variable overrides
├── docker-compose.yml                # Redis + server + worker full stack
├── Dockerfile                        # Multi-stage build (both server + worker binaries)
├── go.mod / go.sum                   # Go module definition
└── .gitignore
```

---

## 4. Architecture Overview

Notifly follows a **Clean / Hexagonal Architecture** pattern with an **async processing pipeline**:

```
┌─────────────────────────────────────────────────────────────┐
│                     cmd/server/main.go                       │
│               (HTTP API + dependency wiring)                 │
├─────────────────────────────────────────────────────────────┤
│                     cmd/worker/main.go                       │
│          (Queue worker + reaper + dependency wiring)         │
├───────────┬─────────────────────────────┬───────────────────┤
│ middleware│    domain/notification       │     common        │
│           │  ┌───────────────────────┐  │                   │
│  auth     │  │    handler.go         │  │  errors.go        │
│  cors     │  │    service.go         │  │  response.go      │
│  ratelimit│  │    worker.go          │  │                   │
│  requestid│  │    reaper.go          │  │                   │
│           │  │    model.go           │  │                   │
│           │  │    log_model.go       │  │                   │
│           │  │    task.go            │  │                   │
│           │  │    provider.go  ◄─────┼──┼── interfaces      │
│           │  │    store.go     ◄─────┼──┼── interfaces      │
│           │  │    ratelimit.go ◄─────┼──┼── interfaces      │
│           │  └───────────────────────┘  │                   │
├───────────┴─────────────────────────────┴───────────────────┤
│                      infra/ (adapters)                       │
│   email/resend.go ──────► implements Provider                │
│   template/engine.go ───► implements TemplateRenderer        │
│   store/supabase.go ────► implements NotificationStore       │
│   queue/asynq.go ───────► asynq client/server wrappers       │
│   ratelimit/recipient.go► implements RecipientRateLimiter    │
└─────────────────────────────────────────────────────────────┘
```

### Key Architectural Principles

1. **Domain defines interfaces, infra implements them.** The `notification` package owns `Provider`, `TemplateRenderer`, `NotificationStore`, and `RecipientRateLimiter` interfaces. Infra packages provide concrete implementations.

2. **Async-first processing.** The API server enqueues notifications to Redis (via asynq). A separate worker process picks them up, renders templates, and sends via providers. This decouples caller latency from provider latency.

3. **Database is the source of truth.** Supabase holds the canonical state of every notification. Redis is a performance layer. If they diverge, the reaper reconciles them.

4. **Manual dependency injection.** All wiring happens in `main.go` (server and worker) — no DI framework.

5. **Typed domain errors.** Domain code returns semantic errors (`ValidationError`, `ProviderError`, etc.) and `common.HandleError` maps them to proper HTTP status codes.

6. **Fail-open for non-critical dependencies.** If Redis is temporarily unreachable, the rate limiter logs the error but allows the request through — never blocking a notification because of a monitoring dependency.

7. **Standardized API envelope.** Every response follows:
   ```json
   {
     "success": true|false,
     "data": { ... },
     "error": { "code": 400, "message": "..." }
   }
   ```

---

## 5. Request Lifecycle (Send Notification — Async)

```
Client ──POST──► /api/v1/send
                    │
           ┌────────▼────────┐
           │  Middleware Stack │
           │  1. Recovery      │
           │  2. RequestID     │
           │  3. CORS          │
           │  4. RateLimit(IP) │
           │  5. Logger        │
           │  6. Auth (API Key)│
           └────────┬─────────┘
                    │
           ┌────────▼─────────┐
           │   handler.Send    │
           │  ShouldBindJSON() │
           └────────┬─────────┘
                    │
           ┌────────▼────────────┐
           │   service.Enqueue    │
           │  1. Validate type    │
           │  2. Check idempotency│
           │  3. Check rate limit │
           │  4. Create log (DB)  │
           │  5. Enqueue to Redis │
           └────────┬────────────┘
                    │
              ◄── 202 Accepted ──►
              {"status": "queued"}

              ═══ ASYNC (worker) ═══

           ┌────────▼────────────┐
           │   worker.ProcessTask │
           │  1. Fetch log from DB│
           │  2. Mark processing  │
           │  3. Render template  │
           │  4. Send via provider│
           │  5. Update log status│
           └─────────────────────┘

              ═══ ASYNC (webhooks) ═══

           ┌─────────────────────┐
           │  Resend Webhook      │
           │  email.delivered →   │
           │  status = delivered  │
           └─────────────────────┘
```

### Request Body

```json
{
  "channel": "email",
  "type": "confirm_signup",
  "to": "user@example.com",
  "idempotency_key": "signup-user123-001",
  "data": {
    "ConfirmationURL": "https://app.example.com/confirm?token=abc123"
  }
}
```

### Success Response (202 Accepted)

```json
{
  "success": true,
  "data": {
    "id": "uuid-of-notification-log",
    "idempotency_key": "signup-user123-001",
    "channel": "email",
    "status": "queued"
  }
}
```

---

## 6. Reliability & Self-Healing

### The Problem

In an async system, tasks can become "orphaned" — stuck in `queued` or `processing` forever — if:
- Redis data is lost (container restart without persistence, volume wipe)
- A worker crashes after dequeuing but before completing a task
- Network partition between worker and Redis

### The Solution: Database Reconciliation (Stale Task Reaper)

The worker runs a **reaper goroutine** that periodically scans Supabase for stuck tasks and re-enqueues them. This is an industry-standard pattern used by companies like Stripe and Shopify.

```
┌──────────────────────────────────────────────┐
│              Reaper Loop (every 5min)         │
│                                              │
│  1. Query Supabase:                          │
│     WHERE status IN ('queued','processing')  │
│     AND updated_at < NOW() - 10min           │
│                                              │
│  2. For each stale task:                     │
│     a. Reset status → queued                 │
│     b. Re-enqueue to Redis                   │
│     c. Log recovery                          │
│                                              │
│  Common case: 0 rows → no-op (nearly free)   │
└──────────────────────────────────────────────┘
```

### Why This Is Safe

- **Idempotent tasks**: Even if a task is accidentally re-processed, the notification won't be sent twice because the worker checks the current status before sending.
- **Partial index**: The reaper query uses a PostgreSQL partial index on `(status, updated_at) WHERE status IN ('queued', 'processing')`, so it only scans the rows that matter — not the entire table.
- **Configurable**: All thresholds are configurable via environment variables.

### Configuration

| Env Var                              | Default | Description                   |
| ------------------------------------ | ------- | ----------------------------- |
| `NOTIFLY_REAPER_INTERVAL_SEC`        | `300`   | How often the reaper runs (5 min) |
| `NOTIFLY_REAPER_STALE_THRESHOLD_SEC` | `600`   | Age before a task is "stale" (10 min) |
| `NOTIFLY_REAPER_BATCH_SIZE`          | `50`    | Max tasks recovered per cycle |

---

## 7. Configuration System

Configuration is loaded with this priority (highest wins):

```
Environment Variables (NOTIFLY_*)  >  .env file  >  config.yaml  >  Defaults
```

### Environment Variable Reference

| Env Var                                    | Config Path                        | Default          |
| ------------------------------------------ | ---------------------------------- | ---------------- |
| `NOTIFLY_SERVER_PORT`                      | `server.port`                      | `8081`           |
| `NOTIFLY_SERVER_MODE`                      | `server.mode`                      | `debug`          |
| `NOTIFLY_AUTH_API_KEYS`                    | `auth.api_keys`                    | `[]`             |
| `NOTIFLY_EMAIL_PROVIDER`                   | `email.provider`                   | `resend`         |
| `NOTIFLY_EMAIL_API_KEY`                    | `email.api_key`                    | `""`             |
| `NOTIFLY_EMAIL_FROM_ADDRESS`               | `email.from_address`               | `""`             |
| `NOTIFLY_EMAIL_FROM_NAME`                  | `email.from_name`                  | `""`             |
| `NOTIFLY_CORS_ALLOWED_ORIGINS`             | `cors.allowed_origins`             | —                |
| `NOTIFLY_RATE_LIMIT_REQUESTS_PER_SECOND`   | `rate_limit.requests_per_second`   | `10`             |
| `NOTIFLY_RATE_LIMIT_BURST`                 | `rate_limit.burst`                 | `20`             |
| `NOTIFLY_REDIS_ADDRESS`                    | `redis.address`                    | `localhost:6379` |
| `NOTIFLY_REDIS_PASSWORD`                   | `redis.password`                   | `""`             |
| `NOTIFLY_REDIS_DB`                         | `redis.db`                         | `0`              |
| `NOTIFLY_SUPABASE_URL`                     | `supabase.url`                     | `""`             |
| `NOTIFLY_SUPABASE_SERVICE_KEY`             | `supabase.service_key`             | `""`             |
| `NOTIFLY_QUEUE_CONCURRENCY`                | `queue.concurrency`                | `10`             |
| `NOTIFLY_QUEUE_MAX_RETRY`                  | `queue.max_retry`                  | `5`              |
| `NOTIFLY_QUEUE_RETRY_DELAY_SEC`            | `queue.retry_delay_sec`            | `30`             |
| `NOTIFLY_RECIPIENT_RATE_LIMIT_MAX_PER_HOUR`| `recipient_rate_limit.max_per_hour`| `3`              |
| `NOTIFLY_REAPER_INTERVAL_SEC`              | `reaper.interval_sec`              | `300`            |
| `NOTIFLY_REAPER_STALE_THRESHOLD_SEC`       | `reaper.stale_threshold_sec`       | `600`            |
| `NOTIFLY_REAPER_BATCH_SIZE`                | `reaper.batch_size`                | `50`             |

> **Note:** `NOTIFLY_AUTH_API_KEYS` supports comma-separated values for multi-app scenarios.

---

## 8. Notification Types & Templates

Each notification type maps to an HTML template in `internal/infra/template/templates/`:

| Type Constant          | Template File              | Default Subject                          | Template Variables            |
| ---------------------- | -------------------------- | ---------------------------------------- | ----------------------------- |
| `confirm_signup`       | `confirm_signup.html`      | Confirm Your Email Address               | `ConfirmationURL`             |
| `invite_user`          | `invite_user.html`         | You've Been Invited                      | `ConfirmationURL`             |
| `magic_link`           | `magic_link.html`          | Your Sign-In Link                        | `ConfirmationURL`             |
| `change_email`         | `change_email.html`        | Confirm Your New Email Address           | `ConfirmationURL`             |
| `reset_password`       | `reset_password.html`      | Reset Your Password                      | `ConfirmationURL`             |
| `reauthentication`     | `reauthentication.html`    | Confirm Your Identity                    | `ConfirmationURL`             |
| `password_changed`     | `password_changed.html`    | Your Password Has Been Changed           | *(informational — no URL)*    |
| `email_changed`        | `email_changed.html`       | Your Email Address Has Been Changed      | *(informational)*             |
| `phone_changed`        | `phone_changed.html`       | Your Phone Number Has Been Changed       | *(informational)*             |
| `identity_linked`      | `identity_linked.html`     | A New Identity Has Been Linked           | *(informational)*             |
| `identity_unlinked`    | `identity_unlinked.html`   | An Identity Has Been Unlinked            | *(informational)*             |

> **Custom Subject:** Pass `"Subject": "My Custom Subject"` in the `data` map to override the default.

---

## 9. API Endpoints

| Method | Path                        | Auth     | Description                                |
| ------ | --------------------------- | -------- | ------------------------------------------ |
| `GET`  | `/health`                   | None     | Health check (returns `ok`)                |
| `POST` | `/api/v1/send`              | API Key  | Enqueue a notification (returns 202)       |
| `GET`  | `/api/v1/notifications`     | API Key  | List notification logs (paginated)         |
| `GET`  | `/api/v1/notifications/:id` | API Key  | Get a specific notification log            |
| `POST` | `/api/v1/webhooks/resend`   | API Key  | Receive Resend delivery webhooks           |

### Authentication

All `/api/v1/*` routes require the `X-API-Key` header.
Keys are validated using **constant-time comparison** (`crypto/subtle`) to prevent timing attacks.

---

## 10. Notification Lifecycle & Statuses

```
queued → processing → sent → delivered
                        ↓         ↓
                      failed    bounced
                                  ↓
                                opened
```

| Status       | Set By    | Meaning                                       |
| ------------ | --------- | --------------------------------------------- |
| `queued`     | Server    | Request accepted, task enqueued to Redis       |
| `processing` | Worker   | Worker picked up the task from Redis           |
| `sent`       | Worker    | Provider accepted the message                 |
| `failed`     | Worker    | Provider rejected or error occurred           |
| `delivered`  | Webhook   | Recipient's mail server accepted the email    |
| `bounced`    | Webhook   | Delivery failed permanently                   |
| `opened`     | Webhook   | Recipient opened the email                    |

---

## 11. Middleware Stack (Order Matters)

```
1. gin.Recovery()          — Panic recovery → 500
2. middleware.RequestID()  — Inject/forward X-Request-ID
3. middleware.CORS()       — CORS headers from config
4. RateLimiter.Middleware()— Per-IP token bucket
5. gin.Logger()            — Structured request logging
6. middleware.Auth()       — API key check (only on /api/v1/*)
```

---

## 12. Error Handling Strategy

| Domain Error Type   | HTTP Status | When Used                                   |
| ------------------- | ----------- | ------------------------------------------- |
| `ValidationError`   | `400`       | Invalid type, bad input, rate limit exceeded |
| `UnauthorizedError` | `401`       | Missing/invalid API key                     |
| `NotFoundError`     | `404`       | Notification log not found                  |
| `ProviderError`     | `502`       | Resend API failure, external service error  |
| *(default)*         | `500`       | Unhandled/unexpected errors                 |

All errors use `errors.As` for unwrapping, so wrapped errors are correctly mapped.

---

## 13. How to Run

### Option A: Docker Compose (Recommended)

This runs **Redis + Server + Worker** all together with one command.

#### Prerequisites

- Docker and Docker Compose installed
- Supabase project set up with the migration SQL executed
- `.env` file configured with real credentials

#### Step 1: Set Up Supabase

Go to your **Supabase Dashboard** → **SQL Editor** → **New Query** and run the migration:

```sql
-- Paste the contents of migrations/001_init.sql
-- This creates the notification_logs table and all indexes
```

#### Step 2: Configure Environment

```bash
cp .env.example .env
```

Edit `.env` and fill in your real values:

```bash
# Required — get these from Supabase Dashboard → Settings → API
NOTIFLY_SUPABASE_URL=https://your-project.supabase.co
NOTIFLY_SUPABASE_SERVICE_KEY=eyJhbGci...your-service-role-key

# Required — get this from Resend Dashboard
NOTIFLY_EMAIL_API_KEY=re_xxxxxxxxxxxxx
NOTIFLY_EMAIL_FROM_ADDRESS=noreply@yourdomain.com
NOTIFLY_EMAIL_FROM_NAME=YourApp

# Required — generate a secure random key
NOTIFLY_AUTH_API_KEYS=your-secret-api-key-here
```

#### Step 3: Build and Run

```bash
docker-compose up --build
```

You should see all three containers start:

```
redis-1   | Ready to accept connections tcp
server-1  | server starting | address :8081
server-1  | supabase store initialized
server-1  | asynq client initialized | redis redis:6379
server-1  | recipient rate limiter initialized | max_per_hour 3
worker-1  | worker starting | concurrency 10 | redis redis:6379
worker-1  | reaper started | interval 5m0s | stale_threshold 10m0s | batch_size 50
```

#### Step 4: Test

```bash
curl -X POST http://localhost:8081/api/v1/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-api-key-here" \
  -d '{
    "channel": "email",
    "type": "confirm_signup",
    "to": "user@example.com",
    "idempotency_key": "test-001",
    "data": {
      "ConfirmationURL": "https://myapp.com/confirm?token=abc123"
    }
  }'
```

Expected: **202 Accepted** → server logs `notification enqueued` → worker logs `notification sent`.

#### Stopping

```bash
# Graceful shutdown (keeps data)
docker-compose down

# Full reset (removes Redis data volume)
docker-compose down -v
```

---

### Option B: Local Development (Without Docker)

Useful for debugging and rapid iteration.

```bash
# Terminal 1: Start Redis
redis-server

# Terminal 2: Start the API server
go run cmd/server/main.go

# Terminal 3: Start the worker
go run cmd/worker/main.go
```

> **Note:** When running locally, `NOTIFLY_REDIS_ADDRESS` should be `localhost:6379`.
> Docker Compose overrides this to `redis:6379` automatically via the `environment` section.

---

### Query Notification Logs

```bash
# List all notifications (paginated)
curl http://localhost:8081/api/v1/notifications \
  -H "X-API-Key: your-secret-api-key-here"

# Filter by status
curl "http://localhost:8081/api/v1/notifications?status=sent" \
  -H "X-API-Key: your-secret-api-key-here"

# Get a specific notification by ID
curl http://localhost:8081/api/v1/notifications/{id} \
  -H "X-API-Key: your-secret-api-key-here"
```

---

## 14. How to Add a New Notification Type

1. **Add the type constant** in `internal/domain/notification/model.go`:
   ```go
   TypeWelcome NotificationType = "welcome"
   ```

2. **Register it as valid** in the `validTypes` map in the same file.

3. **Create the HTML template** at `internal/infra/template/templates/welcome.html`.

4. **Register the template metadata** in `internal/infra/template/engine.go`:
   ```go
   notification.TypeWelcome: {Subject: "Welcome!", TemplateName: "welcome"},
   ```

5. **Done.** No handler, service, or router changes needed.

---

## 15. How to Add a New Channel (e.g., SMS)

1. **Create the provider** at `internal/infra/sms/twilio.go`:
   ```go
   type TwilioProvider struct { ... }
   func (p *TwilioProvider) Channel() notification.Channel { return notification.ChannelSMS }
   func (p *TwilioProvider) Send(ctx context.Context, msg *notification.Message) (string, error) { ... }
   ```

2. **Add config** for the new provider in `config.go` and `config.yaml`.

3. **Wire it in `cmd/worker/main.go`**:
   ```go
   smsProvider := sms.NewTwilioProvider(cfg.SMS.AccountSID, cfg.SMS.AuthToken, cfg.SMS.FromNumber)
   notifWorker := notification.NewWorker(notifStore, tmplEngine, emailProvider, smsProvider)
   ```

4. **Done.** The worker automatically routes based on the `channel` field in the notification log.

---

## 16. Key Design Decisions

| Decision                          | Rationale                                                                           |
| --------------------------------- | ----------------------------------------------------------------------------------- |
| Async queue (asynq + Redis)       | Callers never block on provider latency; automatic retries with exponential backoff  |
| Two binaries (server + worker)    | Scale independently; worker can be scaled horizontally for throughput                |
| Supabase for persistence          | Hosted PostgreSQL with REST API, no infra management needed                          |
| DB as source of truth             | Supabase holds canonical state; Redis is a performance layer, not a durability layer |
| Stale task reaper                 | Self-healing: no notification is permanently lost even if Redis data is wiped         |
| Fail-open rate limiting           | Redis downtime doesn't block notifications — logs error, allows request through      |
| Idempotency via DB unique key     | Simple, reliable deduplication without TTL complexity                                 |
| Redis sliding window rate limit   | Accurate per-recipient throttling without stateful in-memory tracking                 |
| Manual DI over framework          | Small service — framework overhead adds complexity without benefit                    |
| Typed errors over error codes     | `errors.As` chains provide idiomatic Go error handling                                |
| Interface in domain, impl in infra| Dependency inversion — domain never imports infrastructure                            |
| `html/template` over 3rd-party    | Zero dependencies, Go-native, secure by default (auto-escaping)                      |
| Constant-time key comparison      | Prevents timing-based API key enumeration attacks                                    |
| Graceful shutdown                 | In-flight requests/tasks complete before process exits                               |
| JSON structured logging (`slog`)  | Production-ready, machine-parseable, zero-dependency                                  |
| Partial DB index for reaper       | Only indexes `queued`/`processing` rows — tiny index, near-zero performance impact   |
| Docker Compose for local stack    | One command to run Redis + server + worker; same config for dev and CI                |

---

## 17. File-by-File Reference

### Entry Points

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | HTTP API entry point. Wires store → asynq client → rate limiter → service → handler → router. No template/email dependencies (those are worker-only). |
| `cmd/worker/main.go` | Queue worker entry point. Wires store → template engine → provider → worker + reaper. |

### Domain Layer (`internal/domain/notification/`)

| File | Purpose |
|------|---------|
| `model.go` | DTOs: `SendRequest` (with `idempotency_key`), `SendResponse`, `Message`. Enums: `Channel`, `NotificationType`. |
| `log_model.go` | `NotificationLog` struct with full lifecycle timestamps. `ListFilter`, `ListResponse`. |
| `provider.go` | Interfaces: `Provider` (Send + Channel), `TemplateRenderer` (Render). |
| `store.go` | `NotificationStore` interface: Create, GetByID, GetByIdempotencyKey, UpdateStatus, UpdateWebhookStatus, List, ListStale. |
| `ratelimit.go` | `RecipientRateLimiter` interface: Allow. |
| `task.go` | Asynq task type constant and payload serialization helpers. |
| `service.go` | API-side orchestrator: validate → idempotency check → rate limit → create log → enqueue. Also: GetNotification, ListNotifications, HandleWebhookEvent. |
| `worker.go` | Queue task processor: fetch log → mark processing → render template → send via provider → update status. |
| `reaper.go` | Stale task reaper: periodic goroutine that scans DB for stuck tasks and re-enqueues them. |
| `handler.go` | HTTP handlers: `POST /send` (202), `GET /notifications`, `GET /notifications/:id`, `POST /webhooks/resend`. |

### Infrastructure Layer (`internal/infra/`)

| File | Purpose |
|------|---------|
| `email/resend.go` | `ResendProvider` implements `Provider`. HTTP POST to Resend API with Bearer auth. |
| `template/engine.go` | `Engine` implements `TemplateRenderer`. Loads HTML templates at startup. |
| `store/supabase.go` | `SupabaseStore` implements `NotificationStore`. PostgREST queries via Supabase SDK. |
| `queue/asynq.go` | Asynq `Client`, `Server` wrappers. `EnqueueSendNotification` with configurable retry. |
| `ratelimit/recipient.go` | `RedisRecipientLimiter` implements `RecipientRateLimiter`. Redis sorted sets, sliding window. |

### Supporting Layer

| File | Purpose |
|------|---------|
| `internal/config/config.go` | Viper config loader. Structs for all sections including Reaper config. |
| `internal/common/errors.go` | Typed errors: `NotFoundError`, `ValidationError`, `UnauthorizedError`, `ProviderError`. |
| `internal/common/response.go` | `APIResponse` envelope, `Success()`, `Error()`, `HandleError()` helpers. |
| `internal/middleware/auth.go` | API key validation (constant-time). |
| `internal/middleware/cors.go` | CORS policy from config. |
| `internal/middleware/ratelimit.go` | Per-IP token bucket. |
| `internal/middleware/requestid.go` | UUID v4 request ID injection. |
| `internal/router/router.go` | Gin engine: middleware stack + route registration. |

### Database & Ops

| File | Purpose |
|------|---------|
| `migrations/001_init.sql` | Creates `notification_logs` table, all lookup indexes, and partial reaper index. |
| `Dockerfile` | Multi-stage build: both `notifly-server` and `notifly-worker` binaries in one image. |
| `docker-compose.yml` | Full stack: Redis (with AOF persistence) + server + worker, with health checks. |
| `config.yaml` | All default configuration values. |
| `.env.example` | Template for environment variable overrides. |
