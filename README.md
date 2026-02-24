# 🔔 Notifly

A **production-ready async notification microservice** built in Go. Send and track notifications for your apps through a simple API — with automatic retries, duplicate prevention, and self-healing recovery.

---

## ✨ Features

- **📨 Async Processing** — Notifications are queued (Asynq + Redis) and processed in the background. Callers never block on provider latency.
- **📊 Delivery Tracking** — Full lifecycle tracking: `queued → processing → sent → delivered → opened/bounced`.
- **🔁 Automatic Retries** — Failed deliveries are retried with exponential backoff (30s → 60s → 120s → 240s → 480s).
- **🛡️ Idempotency** — Duplicate requests with the same `idempotency_key` are safely deduplicated.
- **⏱️ Per-Recipient Rate Limiting** — Configurable per-recipient throttling prevents accidental spam (Redis sliding window).
- **🔄 Self-Healing** — A stale task reaper automatically recovers orphaned notifications — no message is ever permanently lost.
- **📬 Webhook Support** — Receive delivery status updates from email providers (Resend webhooks).
- **🏗️ Two-Binary Deployment** — Separate `server` (HTTP API) and `worker` (queue processor) for independent scaling.
- **🐳 Docker Ready** — One-command deployment with Docker Compose (Redis + Server + Worker).

---

## 🏛️ Architecture

```
                  ┌─────────────┐
  HTTP Request ──►│   Server    │──► Redis Queue ──► Worker ──► Email Provider (Resend)
                  │  (API + DI) │                   │
                  └──────┬──────┘                   ├── Template Engine
                         │                          ├── Supabase (logs)
                         │                          └── Reaper (self-healing)
                    Supabase DB
                  (source of truth)
```

**Clean Architecture** — Domain defines interfaces, infrastructure implements them. No framework lock-in.

---

## 🚀 Quick Start

### Prerequisites

- **Go 1.25+**
- **Redis** (or Docker)
- **Supabase** project (free tier works)
- **Resend** API key (for email delivery)

### 1. Clone & Configure

```bash
git clone https://github.com/your-username/notifly.git
cd notifly
cp .env.example .env
```

Edit `.env` with your credentials:

```bash
NOTIFLY_SUPABASE_URL=https://your-project.supabase.co
NOTIFLY_SUPABASE_SERVICE_KEY=your-service-role-key
NOTIFLY_EMAIL_API_KEY=re_your_resend_api_key
NOTIFLY_EMAIL_FROM_ADDRESS=noreply@yourdomain.com
NOTIFLY_AUTH_API_KEYS=your-secret-api-key
```

### 2. Set Up Database

Go to **Supabase Dashboard → SQL Editor → New Query** and paste the contents of `migrations/001_init.sql`.

### 3. Run with Docker Compose (Recommended)

```bash
docker-compose up --build
```

This starts **Redis**, **Server** (port 8081), and **Worker** together.

### 3b. Or Run Locally

```bash
# Terminal 1
redis-server

# Terminal 2
go run cmd/server/main.go

# Terminal 3
go run cmd/worker/main.go
```

### 4. Send Your First Notification

```bash
curl -X POST http://localhost:8081/api/v1/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-api-key" \
  -d '{
    "channel": "email",
    "type": "confirm_signup",
    "to": "user@example.com",
    "idempotency_key": "signup-user123",
    "data": {
      "ConfirmationURL": "https://myapp.com/confirm?token=abc"
    }
  }'
```

**Response:** `202 Accepted` → queued for async delivery.

---

## 📡 API Reference

| Method | Endpoint                    | Auth     | Description                         |
| ------ | --------------------------- | -------- | ----------------------------------- |
| `GET`  | `/health`                   | —        | Health check                        |
| `POST` | `/api/v1/send`              | API Key  | Send a notification (async, 202)    |
| `GET`  | `/api/v1/notifications`     | API Key  | List logs (paginated + filterable)  |
| `GET`  | `/api/v1/notifications/:id` | API Key  | Get a specific notification log     |
| `POST` | `/api/v1/webhooks/resend`   | API Key  | Receive Resend delivery webhooks    |

### Authentication

All `/api/v1/*` endpoints require the `X-API-Key` header.

### Send Request Body

```json
{
  "channel": "email",
  "type": "confirm_signup",
  "to": "user@example.com",
  "idempotency_key": "optional-unique-key",
  "data": {
    "ConfirmationURL": "https://..."
  }
}
```

### Notification Types

| Type                | Template Variables     |
| ------------------- | ---------------------- |
| `confirm_signup`    | `ConfirmationURL`      |
| `invite_user`       | `ConfirmationURL`      |
| `magic_link`        | `ConfirmationURL`      |
| `change_email`      | `ConfirmationURL`      |
| `reset_password`    | `ConfirmationURL`      |
| `reauthentication`  | `ConfirmationURL`      |
| `password_changed`  | *(informational)*      |
| `email_changed`     | *(informational)*      |
| `phone_changed`     | *(informational)*      |
| `identity_linked`   | *(informational)*      |
| `identity_unlinked` | *(informational)*      |

### Query Logs

```bash
# List all
curl http://localhost:8081/api/v1/notifications \
  -H "X-API-Key: your-key"

# Filter by status
curl "http://localhost:8081/api/v1/notifications?status=sent&page=1&page_size=20" \
  -H "X-API-Key: your-key"
```

---

## 📂 Project Structure

```
notifly/
├── cmd/
│   ├── server/main.go          # HTTP API entry point
│   └── worker/main.go          # Queue worker + reaper entry point
├── internal/
│   ├── config/                 # Viper-based config loader
│   ├── common/                 # Shared errors & response envelope
│   ├── domain/notification/    # Business logic, models, interfaces
│   ├── infra/                  # Provider implementations
│   │   ├── email/              # Resend API client
│   │   ├── template/           # HTML template engine + 11 templates
│   │   ├── store/              # Supabase persistence
│   │   ├── queue/              # Asynq client/server wrappers
│   │   └── ratelimit/          # Redis per-recipient rate limiter
│   ├── middleware/             # Auth, CORS, rate limit, request ID
│   └── router/                 # Gin route registration
├── migrations/001_init.sql     # Database schema (run in Supabase SQL Editor)
├── docker-compose.yml          # Full stack: Redis + Server + Worker
├── Dockerfile                  # Multi-stage build
├── config.yaml                 # Default configuration
└── .env.example                # Environment variable template
```

---

## ⚙️ Configuration

Configuration priority: **Environment Variables** > `.env` file > `config.yaml` > Defaults.

All env vars use the `NOTIFLY_` prefix.

| Variable                                     | Default          | Description                         |
| -------------------------------------------- | ---------------- | ----------------------------------- |
| `NOTIFLY_SERVER_PORT`                        | `8081`           | HTTP server port                    |
| `NOTIFLY_SERVER_MODE`                        | `debug`          | Gin mode (debug/release)            |
| `NOTIFLY_AUTH_API_KEYS`                      | —                | Comma-separated API keys            |
| `NOTIFLY_EMAIL_API_KEY`                      | —                | Resend API key                      |
| `NOTIFLY_EMAIL_FROM_ADDRESS`                 | —                | Sender email address                |
| `NOTIFLY_EMAIL_FROM_NAME`                    | —                | Sender display name                 |
| `NOTIFLY_REDIS_ADDRESS`                      | `localhost:6379` | Redis connection address            |
| `NOTIFLY_SUPABASE_URL`                       | —                | Supabase project URL                |
| `NOTIFLY_SUPABASE_SERVICE_KEY`               | —                | Supabase service role key           |
| `NOTIFLY_QUEUE_CONCURRENCY`                  | `10`             | Worker concurrency                  |
| `NOTIFLY_QUEUE_MAX_RETRY`                    | `5`              | Max retries per task                |
| `NOTIFLY_RECIPIENT_RATE_LIMIT_MAX_PER_HOUR`  | `3`              | Max notifications per recipient/hr  |
| `NOTIFLY_REAPER_INTERVAL_SEC`                | `300`            | Reaper scan interval (5 min)        |
| `NOTIFLY_REAPER_STALE_THRESHOLD_SEC`         | `600`            | Stale task age threshold (10 min)   |
| `NOTIFLY_REAPER_BATCH_SIZE`                  | `50`             | Max tasks recovered per cycle       |

---

## 🔒 Security

- **API Key Authentication** — Constant-time comparison (`crypto/subtle`) prevents timing attacks
- **No secrets in repo** — `.env` is gitignored; only `.env.example` with placeholders is tracked
- **Response body limits** — HTTP responses from external providers are capped at 1MB
- **Rate limiting** — Both per-IP (token bucket) and per-recipient (Redis sliding window)
- **Graceful shutdown** — In-flight requests and tasks complete before process exits

---

## 🧩 Extending Notifly

### Add a New Notification Type

1. Add the type constant in `internal/domain/notification/model.go`
2. Register it in the `validTypes` map
3. Create the HTML template in `internal/infra/template/templates/`
4. Register the template metadata in `internal/infra/template/engine.go`

**No handler, service, or router changes needed.**

### Add a New Channel (e.g., SMS)

1. Create the provider in `internal/infra/sms/twilio.go` implementing the `Provider` interface
2. Add config for the new provider
3. Wire it in `cmd/worker/main.go`

---

## 🛠️ Tech Stack

| Component           | Technology                     |
| ------------------- | ------------------------------ |
| Language            | Go 1.25                        |
| HTTP Framework      | Gin                            |
| Task Queue          | Asynq + Redis                  |
| Persistence         | Supabase (PostgreSQL)          |
| Email Provider      | Resend                         |
| Templating          | Go `html/template`             |
| Config              | Viper + godotenv               |
| Logging             | `log/slog` (structured JSON)   |
| Containerization    | Docker + Docker Compose        |

---

## 📖 Documentation

See [`study.md`](study.md) for a comprehensive codebase study guide with architecture diagrams, file-by-file reference, and design decisions.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
