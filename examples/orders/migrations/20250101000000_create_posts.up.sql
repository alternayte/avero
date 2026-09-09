CREATE TABLE IF NOT EXISTS posts (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);
