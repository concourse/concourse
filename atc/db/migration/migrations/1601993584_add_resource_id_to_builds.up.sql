
ALTER TABLE builds
ADD COLUMN resource_id integer UNIQUE REFERENCES resources (id) ON DELETE CASCADE;

