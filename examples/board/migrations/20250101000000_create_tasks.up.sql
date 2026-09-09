CREATE TABLE IF NOT EXISTS tasks (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    done       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL
);
