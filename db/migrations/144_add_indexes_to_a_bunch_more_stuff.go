package migrations

import "github.com/concourse/atc/db/migration"

// We pretty much just added an index for every foreign key, plus one on
// containers for plan_id.
//
// We'll check later which ones are wasteful.
//
// We promise.
//
// Useful queries:
//
// Show fkeys without indexes:
//
//     CREATE FUNCTION pg_temp.sortarray(int2[]) returns int2[] as '
//       SELECT ARRAY(
//           SELECT $1[i]
//             FROM generate_series(array_lower($1, 1), array_upper($1, 1)) i
//         ORDER BY 1
//       )
//     ' language sql;

//     SELECT conrelid::regclass, conname, reltuples::bigint
//     FROM pg_constraint
//     JOIN pg_class ON (conrelid = pg_class.oid)
//     WHERE contype = 'f'
//     AND NOT EXISTS (
//       SELECT 1
//       FROM pg_index
//       WHERE indrelid = conrelid
//       AND pg_temp.sortarray(conkey) = pg_temp.sortarray(indkey)
//     )
//     ORDER BY reltuples DESC;
//
// Show size and # of hits for each index:
//
//     SELECT
//       t.tablename,
//       indexname,
//       c.reltuples AS num_rows,
//       pg_size_pretty(pg_relation_size(quote_ident(t.tablename)::text)) AS table_size,
//       pg_size_pretty(pg_relation_size(quote_ident(indexrelname)::text)) AS index_size,
//       CASE WHEN indisunique THEN 'Y'
//         ELSE 'N'
//       END AS UNIQUE,
//       idx_scan AS number_of_scans,
//       idx_tup_read AS tuples_read,
//       idx_tup_fetch AS tuples_fetched
//     FROM pg_tables t
//     LEFT OUTER JOIN pg_class c ON t.tablename=c.relname
//     LEFT OUTER JOIN (
//       SELECT
//         c.relname AS ctablename,
//         ipg.relname AS indexname,
//         x.indnatts AS number_of_columns,
//         idx_scan,
//         idx_tup_read,
//         idx_tup_fetch,
//         indexrelname,
//         indisunique
//       FROM pg_index x
//       JOIN pg_class c ON c.oid = x.indrelid
//       JOIN pg_class ipg ON ipg.oid = x.indexrelid
//       JOIN pg_stat_all_indexes psai ON x.indexrelid = psai.indexrelid
//     ) AS foo ON t.tablename = foo.ctablename
//     WHERE t.schemaname='public'
//     ORDER BY 1, 2;

func AddIndexesToABunchMoreStuff(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE INDEX builds_team_id ON builds (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX independent_build_inputs_job_id ON independent_build_inputs (job_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX independent_build_inputs_version_id ON independent_build_inputs (version_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX next_build_inputs_job_id ON next_build_inputs (job_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX next_build_inputs_version_id ON next_build_inputs (version_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_resource_cache_id ON resource_cache_uses (resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_build_id ON resource_cache_uses (build_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_resource_type_id ON resource_cache_uses (resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_resource_id ON resource_cache_uses (resource_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_caches_resource_config_id ON resource_caches (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_resource_config_id ON resource_config_uses (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_build_id ON resource_config_uses (build_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_resource_type_id ON resource_config_uses (resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_resource_id ON resource_config_uses (resource_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_configs_base_resource_type_id ON resource_configs (base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_configs_resource_cache_id ON resource_configs (resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_resource_config_id ON containers (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_worker_resource_cache_id ON containers (worker_resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_build_id ON containers (build_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_plan_id ON containers (plan_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_team_id ON containers (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_worker_name ON containers (worker_name)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_container_id ON volumes (container_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_parent_id ON volumes (parent_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_team_id ON volumes (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_worker_resource_cache_id ON volumes (worker_resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_worker_base_resource_type_id ON volumes (worker_base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_resource_caches_worker_base_resource_type_id ON worker_resource_caches (worker_base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_resource_caches_resource_cache_id ON worker_resource_caches (resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_base_resource_types_base_resource_type_id ON worker_base_resource_types (base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_base_resource_types_worker_name ON worker_base_resource_types (worker_name)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX workers_team_id ON workers (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_types_pipeline_id ON resource_types (pipeline_id)`)
	if err != nil {
		return err
	}

	return nil
}
