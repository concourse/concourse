CREATE TABLE signing_keys (
    kid text PRIMARY KEY,
    kty text NOT NULL,
    jwk json NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);