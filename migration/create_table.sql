CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    password VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20) UNIQUE NOT NULL,
    solana_pubkey VARCHAR(44) NOT NULL,
    encrypted_priv_key TEXT NOT NULL,
    name VARCHAR(100),
    state_version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS transfers (
    id SERIAL PRIMARY KEY,
    sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_usdt DECIMAL(20,6) NOT NULL,
    fees_usdt DECIMAL(20,6) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    solana_tx_hash VARCHAR(88) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    claimed_at TIMESTAMP
);

CREATE INDEX idx_transfers_sender ON transfers(sender_id);
CREATE INDEX idx_transfers_recipient ON transfers(recipient_id);
CREATE INDEX idx_transfers_status ON transfers(status);

CREATE TABLE IF NOT EXISTS withdrawals (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_fcfa DECIMAL(20,0) NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    provider VARCHAR(10),
    api_tx_id VARCHAR(100),
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_withdrawals_user ON withdrawals(user_id);