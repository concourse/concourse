BEGIN;
   DROP TABLE build_inputs;

   DROP TABLE build_outputs;

   DROP TABLE versioned_resources;

   ALTER TABLE resources
     DROP COLUMN paused;
COMMIT;
