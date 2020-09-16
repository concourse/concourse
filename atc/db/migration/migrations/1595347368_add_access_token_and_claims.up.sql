BEGIN;
    CREATE TABLE access_tokens (
        token text NOT NULL PRIMARY KEY,
        sub text NOT NULL,
        claims jsonb,
        expires_at timestamp with time zone
    );
COMMIT;
