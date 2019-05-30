BEGIN;
  CREATE TABLE cert_cache (
    "domain" text PRIMARY KEY,
    "cert" text NOT NULL,
    "nonce" text
  );
COMMIT;
