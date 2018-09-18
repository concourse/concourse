BEGIN;
  ALTER TABLE builds ADD COLUMN tracked_by text;

  DROP MATERIALIZED VIEW transition_builds_per_job;
  DROP MATERIALIZED VIEW next_builds_per_job;
  DROP MATERIALIZED VIEW latest_completed_builds_per_job;

  CREATE MATERIALIZED VIEW latest_completed_builds_per_job AS
   WITH latest_build_ids_per_job AS (
           SELECT max(b_1.id) AS build_id
             FROM (builds b_1
               JOIN jobs j ON ((j.id = b_1.job_id)))
            WHERE (b_1.status <> ALL (ARRAY['pending'::build_status, 'started'::build_status]))
            GROUP BY b_1.job_id
          )
   SELECT b.id,
      b.name,
      b.status,
      b.scheduled,
      b.start_time,
      b.end_time,
      b.engine,
      b.engine_metadata,
      b.completed,
      b.job_id,
      b.reap_time,
      b.team_id,
      b.manually_triggered,
      b.interceptible,
      b.nonce,
      b.public_plan,
      b.pipeline_id,
      b.tracked_by
     FROM (builds b
       JOIN latest_build_ids_per_job l ON ((l.build_id = b.id)))
    WITH NO DATA;
  CREATE UNIQUE INDEX latest_completed_builds_per_job_id ON latest_completed_builds_per_job USING btree (id);
  REFRESH MATERIALIZED VIEW latest_completed_builds_per_job;

  CREATE MATERIALIZED VIEW next_builds_per_job AS
   WITH latest_build_ids_per_job AS (
           SELECT min(b_1.id) AS build_id
             FROM (builds b_1
               JOIN jobs j ON ((j.id = b_1.job_id)))
            WHERE (b_1.status = ANY (ARRAY['pending'::build_status, 'started'::build_status]))
            GROUP BY b_1.job_id
          )
   SELECT b.id,
      b.name,
      b.status,
      b.scheduled,
      b.start_time,
      b.end_time,
      b.engine,
      b.engine_metadata,
      b.completed,
      b.job_id,
      b.reap_time,
      b.team_id,
      b.manually_triggered,
      b.interceptible,
      b.nonce,
      b.public_plan,
      b.pipeline_id,
      b.tracked_by
     FROM (builds b
       JOIN latest_build_ids_per_job l ON ((l.build_id = b.id)))
    WITH NO DATA;
  CREATE UNIQUE INDEX next_builds_per_job_id ON next_builds_per_job USING btree (id);
  REFRESH MATERIALIZED VIEW next_builds_per_job;

  CREATE MATERIALIZED VIEW transition_builds_per_job AS
   WITH builds_before_transition AS (
           SELECT b_1.job_id,
              max(b_1.id) AS max
             FROM ((builds b_1
               LEFT JOIN jobs j ON ((b_1.job_id = j.id)))
               LEFT JOIN latest_completed_builds_per_job s ON ((b_1.job_id = s.job_id)))
            WHERE ((b_1.status <> s.status) AND (b_1.status <> ALL (ARRAY['pending'::build_status, 'started'::build_status])))
            GROUP BY b_1.job_id
          )
   SELECT DISTINCT ON (b.job_id) b.id,
      b.name,
      b.status,
      b.scheduled,
      b.start_time,
      b.end_time,
      b.engine,
      b.engine_metadata,
      b.completed,
      b.job_id,
      b.reap_time,
      b.team_id,
      b.manually_triggered,
      b.interceptible,
      b.nonce,
      b.public_plan,
      b.pipeline_id,
      b.tracked_by
     FROM (builds b
       LEFT JOIN builds_before_transition ON ((b.job_id = builds_before_transition.job_id)))
    WHERE (((builds_before_transition.max IS NULL) AND (b.status <> ALL (ARRAY['pending'::build_status, 'started'::build_status]))) OR (b.id > builds_before_transition.max))
    ORDER BY b.job_id, b.id
    WITH NO DATA;
  CREATE UNIQUE INDEX transition_builds_per_job_id ON transition_builds_per_job USING btree (id);
  REFRESH MATERIALIZED VIEW transition_builds_per_job;
COMMIT;
