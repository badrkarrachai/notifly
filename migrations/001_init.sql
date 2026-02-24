-- Notifly: Database Schema
-- Run this in Supabase SQL Editor (Dashboard → SQL Editor → New Query)

-- =============================================
-- notification_logs — stores every notification
-- =============================================
CREATE TABLE IF NOT EXISTS notification_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE,
    channel         VARCHAR(20)  NOT NULL,
    type            VARCHAR(50)  NOT NULL,
    recipient       VARCHAR(255) NOT NULL,
    template_data   JSONB,
    provider_id     VARCHAR(255),
    status          VARCHAR(20)  NOT NULL DEFAULT 'queued',
    error_message   TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    opened_at       TIMESTAMPTZ,
    bounced_at      TIMESTAMPTZ
);

-- =============================================
-- Indexes
-- =============================================

-- Lookup by recipient (for rate limiting queries / list filtering)
CREATE INDEX IF NOT EXISTS idx_notif_logs_recipient ON notification_logs(recipient);

-- Lookup by status (for list filtering)
CREATE INDEX IF NOT EXISTS idx_notif_logs_status ON notification_logs(status);

-- Lookup by idempotency key (for deduplication)
CREATE INDEX IF NOT EXISTS idx_notif_logs_idempotency ON notification_logs(idempotency_key);

-- Ordering by creation time (for paginated listing)
CREATE INDEX IF NOT EXISTS idx_notif_logs_created ON notification_logs(created_at);

-- Partial index for the stale task reaper: only indexes queued/processing rows
-- so the periodic reconciliation query is near-zero cost
CREATE INDEX IF NOT EXISTS idx_notif_logs_stale
    ON notification_logs (status, updated_at)
    WHERE status IN ('queued', 'processing');
