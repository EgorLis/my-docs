CREATE TABLE IF NOT EXISTS app.subscriptions (
    id              TEXT PRIMARY KEY,
    service_name    TEXT NOT NULL,
    price           INTEGER NOT NULL,
    user_id         TEXT NOT NULL,
    start_date      TIMESTAMPTZ NOT NULL,
    end_date        TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON app.subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_service ON app.subscriptions(service_name);
CREATE INDEX IF NOT EXISTS idx_subscriptions_period ON app.subscriptions(start_date, end_date);