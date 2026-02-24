# 🔒 Notifly — Codebase Rules

> **Purpose:** Mandatory rules for all code contributions to Notifly — by humans and AI alike.
> Violating these rules introduces bugs, tech debt, or security issues. No exceptions.

---

## 1. Architecture Rules

### 1.1 — Domain Never Imports Infra

```
internal/domain/  →  MUST NOT import  →  internal/infra/
internal/domain/  →  MUST NOT import  →  internal/middleware/
internal/domain/  →  CAN import       →  internal/common/
```

The `domain/` package defines **interfaces** (`Provider`, `TemplateRenderer`). Infrastructure packages implement them. If you need external functionality in the domain layer, define an interface — never import the concrete implementation.

### 1.2 — All Wiring Happens in `main.go`

Dependencies are wired manually in `cmd/server/main.go`. Do **not** use init() functions, global singletons, or service locators. Every dependency must be explicitly constructed and injected.

### 1.3 — One Domain per Directory

Each domain concept gets its own directory under `internal/domain/`. A domain directory contains exactly:
- `model.go` — DTOs, enums, validation helpers
- `provider.go` — Interface definitions (ports)
- `service.go` — Business logic orchestration
- `handler.go` — HTTP handler (thin adapter)

Do **not** merge these into a single file or split them differently.

---

## 2. Error Handling Rules

### 2.1 — Always Use Typed Domain Errors

Never return raw strings or `fmt.Errorf` as final errors from domain code. Always use the typed errors from `internal/common/errors.go`:

```go
// ✅ CORRECT
return nil, common.NewValidationError("unsupported notification type: " + string(t))

// ❌ WRONG
return nil, fmt.Errorf("unsupported notification type: %s", t)
```

`fmt.Errorf` with `%w` wrapping is acceptable **only** when wrapping a typed error or a lower-level error from infrastructure code.

### 2.2 — Never Swallow Errors

Every error must be either:
1. **Returned** to the caller, or
2. **Logged** with `slog.Error(...)` and handled with a proper response

```go
// ❌ NEVER DO THIS
_ = someFunction()

// ❌ NEVER DO THIS
if err != nil {
    // silently continue
}
```

### 2.3 — Handler Error Mapping

Handlers must delegate error-to-HTTP-status mapping to `common.HandleError(c, err)`. Do **not** manually map domain errors to status codes in handlers.

```go
// ✅ CORRECT
if err != nil {
    common.HandleError(c, err)
    return
}

// ❌ WRONG
if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}
```

### 2.4 — Never Expose Internal Errors to Clients

Provider and infrastructure errors must be wrapped with a generic message in the response. Internal details go to **logs only**.

```go
// ✅ CORRECT — in response.go
case errors.As(err, &provider):
    Error(c, http.StatusBadGateway, "notification delivery failed")  // generic

// ❌ WRONG
Error(c, http.StatusBadGateway, provider.Error())  // leaks internals
```

---

## 3. API & Response Rules

### 3.1 — Standardized Response Envelope

Every response **must** use the `common.Success()` or `common.Error()` helpers. Never use `c.JSON()` directly with custom structures.

```go
// ✅ CORRECT
common.Success(c, http.StatusOK, data)

// ❌ WRONG
c.JSON(http.StatusOK, gin.H{"status": "ok", "data": data})
```

### 3.2 — Correct HTTP Status Codes

| Situation                        | Status Code |
| -------------------------------- | ----------- |
| Success                          | `200`       |
| Resource created                 | `201`       |
| Invalid input / validation error | `400`       |
| Missing or invalid API key       | `401`       |
| Resource not found               | `404`       |
| Rate limit exceeded              | `429`       |
| External provider failure        | `502`       |
| Unexpected internal error        | `500`       |

### 3.3 — Use Gin Binding Tags for Validation

Validation belongs in struct tags, not in handler code. Use `binding:"required"`, `binding:"oneof=..."`, etc.

```go
// ✅ CORRECT
Channel Channel `json:"channel" binding:"required,oneof=email sms push"`

// ❌ WRONG — manual validation in handler
if req.Channel == "" {
    common.Error(c, 400, "channel is required")
}
```

---

## 4. Configuration Rules

### 4.1 — Never Hardcode Configuration Values

All configurable values (ports, API keys, URLs, timeouts) must come from `internal/config/config.go`. Never hardcode them in business logic.

```go
// ✅ CORRECT
httpClient: &http.Client{Timeout: 10 * time.Second},  // acceptable — infrastructure constant

// ❌ WRONG
apiKey := "re_my_resend_key_123"
```

### 4.2 — Environment Variable Naming Convention

All environment variables must:
- Use the `NOTIFLY_` prefix
- Use SCREAMING_SNAKE_CASE
- Map to nested config using underscores: `NOTIFLY_EMAIL_API_KEY` → `email.api_key`

### 4.3 — Secrets Must Never Be Committed

The `.env` file is gitignored. Secrets must only live in:
- `.env` file (local development)
- Environment variables (CI/CD, Docker)
- Secret manager (production)

**Never** put real secrets in `config.yaml` or commit them to Git.

---

## 5. Middleware Rules

### 5.1 — Middleware Order is Fixed

The middleware stack in `router/router.go` has a deliberate order. Do **not** rearrange it unless you fully understand the implications:

```
1. Recovery      — must be first (catches panics from everything below)
2. RequestID     — inject ID before any logging happens
3. CORS          — handle preflight before auth rejects it
4. RateLimiter   — reject excessive traffic before auth processing
5. Logger        — log all requests (including rejected ones)
6. Auth          — applied only to protected route groups
```

### 5.2 — Auth Middleware Uses Constant-Time Comparison

API key validation **must** use `crypto/subtle.ConstantTimeCompare`. Never use `==` for secret comparison — it's vulnerable to timing attacks.

### 5.3 — New Middleware Must Accept Dependencies via Parameters

Do not use globals or `init()` in middleware. Pass all configuration as function parameters.

```go
// ✅ CORRECT
func Auth(validKeys []string) gin.HandlerFunc { ... }

// ❌ WRONG
var globalKeys []string
func Auth() gin.HandlerFunc { ... }  // reads globalKeys
```

---

## 6. Template Rules

### 6.1 — One Template Per Notification Type

Every notification type in `model.go` must have a corresponding `.html` template in `internal/infra/template/templates/`.

### 6.2 — Template Registration is Mandatory

Adding a template file without registering it in the `registry` map in `engine.go` is a bug. Both must always be in sync.

### 6.3 — Templates Must Be Email-Client Compatible

HTML email templates must:
- Use **table-based layouts** (not CSS grid/flexbox)
- Use **inline styles** (not `<style>` blocks or external CSS)
- Include proper `<meta charset="utf-8">` and viewport tags
- Avoid JavaScript (email clients strip it)

### 6.4 — Template Variables Use Go Template Syntax

```html
<!-- ✅ CORRECT -->
<a href="{{.ConfirmationURL}}">

<!-- ❌ WRONG -->
<a href="${confirmationUrl}">
```

---

## 7. Provider / Infrastructure Rules

### 7.1 — Providers Must Implement the Provider Interface

Every new channel provider must satisfy `notification.Provider`. Use the compile-time check:

```go
var _ notification.Provider = (*MyProvider)(nil)
```

### 7.2 — Providers Must Propagate Context

All external calls must use the `context.Context` parameter for cancellation and timeout propagation.

```go
// ✅ CORRECT
req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)

// ❌ WRONG
req, err := http.NewRequest(http.MethodPost, url, body)
```

### 7.3 — HTTP Clients Must Have Timeouts

Every `http.Client` used to call external APIs must set a timeout. Default: **10 seconds**.

```go
httpClient: &http.Client{Timeout: 10 * time.Second},
```

Never use `http.DefaultClient` — it has no timeout.

### 7.4 — Always Close Response Bodies

External HTTP response bodies must always be closed:

```go
defer resp.Body.Close()
```

---

## 8. Logging Rules

### 8.1 — Use `log/slog` Exclusively

Never use `fmt.Println`, `log.Println`, or third-party loggers. All logging goes through `slog`:

```go
slog.Info("notification sent", "channel", req.Channel, "type", req.Type)
slog.Error("delivery failed", "error", err, "duration", time.Since(start))
```

### 8.2 — Structured Key-Value Pairs

All log entries must use key-value pairs, not formatted strings:

```go
// ✅ CORRECT
slog.Error("send failed", "channel", req.Channel, "error", err)

// ❌ WRONG
slog.Error(fmt.Sprintf("send failed for channel %s: %v", req.Channel, err))
```

### 8.3 — Never Log Sensitive Data

Never log:
- API keys or secrets
- Full email addresses in production (consider masking)
- Request/response bodies containing PII

---

## 9. Naming Conventions

### 9.1 — Go Standard Naming

| Item               | Convention            | Example                    |
| ------------------ | --------------------- | -------------------------- |
| Package names      | lowercase, short      | `notification`, `email`    |
| Exported functions | PascalCase            | `NewService`, `Send`       |
| Unexported funcs   | camelCase             | `isValidKey`, `stripHTML`  |
| Constants (enum)   | PascalCase with prefix| `ChannelEmail`, `TypeResetPassword` |
| Interfaces         | Noun or -er suffix    | `Provider`, `TemplateRenderer` |
| Error types        | `XxxError`            | `ValidationError`, `ProviderError` |

### 9.2 — File Naming

- All Go files use `snake_case.go`
- Template files match their notification type: `confirm_signup.html`
- Config files use `config.yaml` and `.env`

### 9.3 — JSON Field Naming

All JSON fields use `snake_case`:

```go
Channel Channel `json:"channel"`
Type    NotificationType `json:"type"`
```

---

## 10. Concurrency & Safety Rules

### 10.1 — Protect Shared State with Mutexes

The rate limiter uses a double-checked locking pattern with `sync.RWMutex`. Any new shared mutable state must follow the same or an equivalent pattern.

### 10.2 — Use Context for Cancellation

All operations that may block (HTTP calls, database queries) must accept and respect `context.Context`.

---

## 11. Docker / Deployment Rules

### 11.1 — Multi-Stage Builds Only

The Dockerfile must use multi-stage builds:
1. **Builder stage**: compile with the full Go toolchain
2. **Runtime stage**: minimal Alpine image with only the binary

### 11.2 — Templates Must Be Copied to Runtime Image

The Dockerfile must copy templates to `/app/templates`. The `resolveTemplatesDir()` function in `main.go` checks for this path first.

### 11.3 — CGO Must Be Disabled

The binary must be compiled with `CGO_ENABLED=0` for Alpine compatibility:

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -o notifly cmd/server/main.go
```

---

## 12. Adding New Features Checklist

### New Notification Type
- [ ] Add `TypeXxx` constant in `model.go`
- [ ] Register in `validTypes` map in `model.go`
- [ ] Create `xxx.html` template in `templates/`
- [ ] Register in `registry` map in `engine.go`
- [ ] Update `study.md` notification types table

### New Channel Provider
- [ ] Create package under `internal/infra/` (e.g., `sms/`)
- [ ] Implement `notification.Provider` interface
- [ ] Add compile-time interface check: `var _ notification.Provider = (*XxxProvider)(nil)`
- [ ] Add config struct and fields in `config.go`
- [ ] Add env var mappings in `config.yaml` and `.env.example`
- [ ] Wire in `main.go`
- [ ] Update `study.md` architecture section

### New Middleware
- [ ] Create in `internal/middleware/`
- [ ] Accept all config via function parameters (no globals)
- [ ] Add to router stack in `router.go` at the correct position
- [ ] Document the middleware order in `study.md`

### New Endpoint
- [ ] Add handler method in the appropriate domain handler
- [ ] Register route via `RegisterRoutes()` method
- [ ] Use `common.Success()` / `common.HandleError()` for responses
- [ ] Add to API endpoints table in `study.md`
