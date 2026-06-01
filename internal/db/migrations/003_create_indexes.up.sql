-- 003_create_indexes.sql
-- Performance indexes for common dashboard query patterns.

-- llm_calls: queries filter heavily by provider, model, timestamp, and status.
CREATE INDEX IF NOT EXISTS idx_llm_calls_provider
    ON llm_calls (provider, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_calls_model
    ON llm_calls (model, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_calls_status
    ON llm_calls (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_calls_provider_model
    ON llm_calls (provider, model, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_llm_calls_created_at
    ON llm_calls (created_at DESC);

-- llm_metrics_hourly: queries filter by hour_bucket and provider.
CREATE INDEX IF NOT EXISTS idx_llm_metrics_hourly_bucket
    ON llm_metrics_hourly (hour_bucket DESC);

CREATE INDEX IF NOT EXISTS idx_llm_metrics_hourly_provider
    ON llm_metrics_hourly (provider, hour_bucket DESC);

CREATE INDEX IF NOT EXISTS idx_llm_metrics_hourly_model
    ON llm_metrics_hourly (model, hour_bucket DESC);
