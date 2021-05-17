CREATE TABLE prototypes (
    id serial PRIMARY KEY,
    pipeline_id integer NOT NULL REFERENCES pipelines (id) ON DELETE CASCADE,
    name text NOT NULL,
    type text NOT NULL,
    config text,
    active boolean DEFAULT false NOT NULL,
    nonce text,
    resource_config_id integer REFERENCES resource_configs (id)
);

CREATE UNIQUE INDEX prototypes_pipeline_id_name_uniq
    ON prototypes (pipeline_id, name);

CREATE INDEX prototypes_pipeline_id
    ON prototypes (pipeline_id);

CREATE INDEX prototypes_resource_config_id
    ON prototypes (resource_config_id);

ALTER TABLE builds
    ADD COLUMN prototype_id integer REFERENCES prototypes (id) ON DELETE CASCADE;

CREATE INDEX builds_prototype_id_idx ON builds (prototype_id);
