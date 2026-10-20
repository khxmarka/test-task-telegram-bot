-- +goose Up
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    telegram_id     BIGINT NOT NULL UNIQUE,
    display_name    TEXT NOT NULL,
    username        TEXT,
    is_family_head  BOOLEAN NOT NULL DEFAULT FALSE,
    language        TEXT NOT NULL DEFAULT 'ru' CHECK (language IN ('ru', 'en')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE expenses (
    id                   BIGSERIAL PRIMARY KEY,
    user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_byn_kopecks   BIGINT NOT NULL CHECK (amount_byn_kopecks > 0),
    category             TEXT NOT NULL CHECK (category IN (
        'stores', 'cafes', 'transport', 'housing', 'entertainment',
        'health', 'shopping', 'household', 'other'
    )),
    spent_at             DATE NOT NULL,
    comment              TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_expenses_user_spent_created
    ON expenses (user_id, spent_at DESC, created_at DESC);

CREATE INDEX idx_expenses_category ON expenses (category);
CREATE INDEX idx_expenses_spent_at ON expenses (spent_at DESC);

-- +goose Down
DROP TABLE IF EXISTS expenses;
DROP TABLE IF EXISTS users;
