BEGIN;
    -- for all of these, just set an empty config and a NULL nonce value so that
    -- the encryption can be performed on start

    UPDATE jobs SET config = '{}', nonce = NULL WHERE config IS NULL;
    ALTER TABLE jobs ALTER COLUMN config SET NOT NULL;

    UPDATE resources SET config = '{}', nonce = NULL WHERE config IS NULL;
    ALTER TABLE resources ALTER COLUMN config SET NOT NULL;

    UPDATE resource_types SET config = '{}', nonce = NULL WHERE config IS NULL;
    ALTER TABLE resource_types ALTER COLUMN config SET NOT NULL;
COMMIT;