-- Orchestra chat: shared message channel for director, musicians, and humans.

CREATE TABLE IF NOT EXISTS orchestra_chat (
    id              TEXT PRIMARY KEY,
    orchestra_id    TEXT NOT NULL REFERENCES orchestras(id) ON DELETE CASCADE,
    sender_type     VARCHAR(20) NOT NULL CHECK (sender_type IN ('system', 'director', 'musician', 'human')),
    sender_id       TEXT,
    message_type    VARCHAR(20) NOT NULL DEFAULT 'text'
                    CHECK (message_type IN ('text', 'result', 'request_help', 'action', 'choice', 'choice_response', 'file', 'status', 'warning')),
    content         TEXT NOT NULL,
    data            JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_orchestra ON orchestra_chat(orchestra_id, created_at);
