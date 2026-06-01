-- 001_create_llm_calls.sql
-- Creates the main call history table, partitioned by day using PostgreSQL
-- declarative range partitioning on the created_at column.

CREATE TABLE IF NOT EXISTS llm_calls (
    id            BIGSERIAL,
    event_id      UUID        NOT NULL,
    provider      VARCHAR(32) NOT NULL,
    model         VARCHAR(128) NOT NULL,
    latency_ms    BIGINT      NOT NULL DEFAULT 0,
    input_tokens  INTEGER     NOT NULL DEFAULT 0,
    output_tokens INTEGER     NOT NULL DEFAULT 0,
    cost_usd      NUMERIC(18, 8) NOT NULL DEFAULT 0,
    status        VARCHAR(16) NOT NULL,
    error_message TEXT,
    event_ts      TIMESTAMPTZ NOT NULL, -- original event timestamp
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Default partition catches anything that doesn't match a specific day partition.
-- Day partitions are created dynamically by the application on startup.
CREATE TABLE IF NOT EXISTS llm_calls_default
    PARTITION OF llm_calls DEFAULT;

-- Unique constraint on event_id to support idempotent inserts.
-- Note: unique constraints on partitioned tables require the partition key.
CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_calls_event_id
    ON llm_calls (event_id, created_at);
