
-- +migrate Up
CREATE TABLE urls (
    id bigserial PRIMARY KEY,
    short_url VARCHAR(20) UNIQUE NOT NULL,
    long_url VARCHAR(255) NOT NULL,
    expire_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +migrate Down
DROP TABLE IF EXISTS users;
