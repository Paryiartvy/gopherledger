CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    login TEXT UNIQUE,
    password_hash TEXT NOT NULL
);

CREATE TABLE auth (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL
);

CREATE TABLE balances (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current NUMERIC(13,3) NOT NULL DEFAULT 0.000 CHECK ( current >= 0 ),
    withdrawn NUMERIC(13,3) NOT NULL DEFAULT 0.000
);

CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    number TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    accrual NUMERIC(13,3) NOT NULL DEFAULT 0.000,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_data ON orders(user_id, uploaded_at DESC);

CREATE INDEX idx_orders_unprocessed ON orders(status)
WHERE status IN('NEW', 'PROCESSING');

CREATE TABLE withdrawals (
     id BIGSERIAL PRIMARY KEY,
     user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
     order_number TEXT NOT NULL UNIQUE,
     sum NUMERIC(13,3) NOT NULL DEFAULT 0.000,
     processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdrawals_user_data ON withdrawals(user_id, processed_at DESC);