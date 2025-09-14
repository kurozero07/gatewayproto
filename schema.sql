CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    token VARCHAR(64) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_transactions_created_at ON transactions(created_at);