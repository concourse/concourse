BEGIN;
  ALTER TABLE pipelines DROP COLUMN cache_index;
  ALTER TABLE versioned_resources ADD COLUMN modified_time timestamp without time zone DEFAULT now() NOT NULL;
  ALTER TABLE build_inputs ADD COLUMN modified_time timestamp without time zone DEFAULT now() NOT NULL;
  ALTER TABLE build_outputs ADD COLUMN modified_time timestamp without time zone DEFAULT now() NOT NULL;
COMMIT;
