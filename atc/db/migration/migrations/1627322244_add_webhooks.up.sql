CREATE TABLE webhooks (
  id serial PRIMARY KEY,
  name text NOT NULL,
  team_id integer NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
  type text NOT NULL,
  token text NOT NULL,
  nonce text
);

CREATE UNIQUE INDEX webhooks_name_team_id ON webhooks (name, team_id);

CREATE TABLE resource_webhooks (
  id serial PRIMARY KEY,
  resource_id integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
  webhook_type text NOT NULL,
  webhook_filter jsonb NOT NULL
);

CREATE INDEX resource_webhooks_resource_id ON resource_webhooks (resource_id);
CREATE INDEX resource_webhooks_webhook_type ON resource_webhooks (webhook_type);
