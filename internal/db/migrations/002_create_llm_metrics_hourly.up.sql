-- 002_create_llm_metrics_hourly.sql
-- Pre-aggregated hourly rollup table for fast dashboard queries.
-- Populated by the Kafka consumer's hourly rollup job.

CREATE TABLE IF NOT EXISTS llm_metrics_hourly (
    id            BIGSERIAL PRIMARY KEY,
    hour_bucket   TIMESTAMPTZ NOT NULL, -- truncated to the hour
    provider      VARCHAR(32) NOT NULL,
    model         VARCHAR(128) NOT NULL,
    total_calls   BIGINT      NOT NULL DEFAULT 0,
    success_calls BIGINT      NOT NULL DEFAULT 0,
    error_calls   BIGINT      NOT NULL DEFAULT 0,
    total_cost_usd NUMERIC(18, 8) NOT NULL DEFAULT 0,
    total_latency_ms BIGINT   NOT NULL DEFAULT 0,
    min_latency_ms   BIGINT   NOT NULL DEFAULT 0,
    max_latency_ms   BIGINT   NOT NULL DEFAULT 0,
    total_input_tokens  BIGINT NOT NULL DEFAULT 0,
    total_output_tokens BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (hour_bucket, provider, model)
);

-- Trigger to auto-update updated_at on upsert.
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_llm_metrics_hourly_updated_at
    BEFORE UPDATE ON llm_metrics_hourly
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
