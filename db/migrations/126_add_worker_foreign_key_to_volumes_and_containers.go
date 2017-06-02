package migrations

import "github.com/concourse/atc/db/migration"

func AddWorkerForeignKeyToVolumesAndContainers(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM volumes v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM volumes v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM containers v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM containers v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ADD CONSTRAINT volumes_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD CONSTRAINT containers_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name);
	`)
	if err != nil {
		return err
	}

	return nil
}
