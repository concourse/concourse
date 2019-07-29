package migrations

import (
	"database/sql"
	"encoding/json"
)

type ResourceConfig struct {
	Name         string   `json:"name"`
	Public       bool     `json:"public,omitempty"`
	WebhookToken string   `json:"webhook_token,omitempty"`
	Type         string   `json:"type"`
	Source       Source   `json:"source"`
	CheckEvery   string   `json:"check_every,omitempty"`
	CheckTimeout string   `json:"check_timeout,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Version      Version  `json:"version,omitempty"`
}

type Source map[string]interface{}

type Params map[string]interface{}

type Version map[string]string

func (self *migrations) Down_1537546150() error {
	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, config, nonce FROM resources`)
	if err != nil {
		return err
	}

	type resource struct {
		id    int
		type_ string
	}

	resources := []resource{}
	for rows.Next() {
		var (
			configBlob string
			nonce      sql.NullString
		)

		r := resource{}
		if err = rows.Scan(&r.id, &configBlob, &nonce); err != nil {
			return err
		}

		var noncense *string
		if nonce.Valid {
			noncense = &nonce.String
		}

		decryptedConfig, err := self.Decrypt(string(configBlob), noncense)
		if err != nil {
			return err
		}

		var config ResourceConfig
		err = json.Unmarshal(decryptedConfig, &config)
		if err != nil {
			return err
		}

		r.type_ = config.Type

		resources = append(resources, r)
	}

	for _, r := range resources {
		_, err = tx.Exec(`INSERT INTO versioned_resources (version, metadata, type, resource_id, check_order, enabled)
	 SELECT rcv.version, rcv.metadata, $2, r.id, rcv.check_order,
		NOT EXISTS ( SELECT 1 FROM resource_disabled_versions d WHERE d.version_md5 = rcv.version_md5 AND d.resource_id = r.id )
	 FROM resource_config_versions rcv, resources r
	 WHERE r.resource_config_id = rcv.resource_config_id AND r.id = $1
	 ON CONFLICT (resource_id, md5(version), type) DO UPDATE SET
		metadata = EXCLUDED.metadata,
		check_order = EXCLUDED.check_order,
		enabled = EXCLUDED.enabled
	 `, r.id, r.type_)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	_, err = tx.Exec(`ALTER TABLE resources
    ADD COLUMN last_checked timestamp with time zone,
		DROP CONSTRAINT resources_resource_config_id_fkey,
    ADD CONSTRAINT resources_resource_config_id_fkey FOREIGN KEY (resource_config_id) REFERENCES resource_configs(id) ON DELETE SET NULL`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE resource_types
    ADD COLUMN last_checked timestamp with time zone DEFAULT '1970-01-01 00:00:00' NOT NULL,
		ADD COLUMN version text`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE resource_configs DROP COLUMN last_checked,
		DROP COLUMN check_error`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE resource_types DROP COLUMN check_error`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DROP TABLE build_resource_config_version_inputs`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DROP TABLE build_resource_config_version_outputs`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`TRUNCATE TABLE next_build_inputs`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE next_build_inputs DROP COLUMN resource_config_version_id,
		DROP COLUMN resource_id,
		ADD COLUMN version_id integer NOT NULL`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`CREATE INDEX next_build_inputs_version_id ON next_build_inputs USING btree (version_id)`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE ONLY next_build_inputs
      ADD CONSTRAINT next_build_inputs_version_id_fkey FOREIGN KEY (version_id) REFERENCES versioned_resources(id) ON DELETE CASCADE`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`TRUNCATE TABLE independent_build_inputs`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE independent_build_inputs DROP COLUMN resource_config_version_id, DROP COLUMN resource_id, ADD COLUMN version_id integer NOT NULL`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`CREATE INDEX independent_build_inputs_version_id ON independent_build_inputs USING btree (version_id)`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_version_id_fkey FOREIGN KEY (version_id) REFERENCES versioned_resources(id) ON DELETE CASCADE`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DROP TABLE resource_config_versions`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DROP TABLE resource_disabled_versions`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DROP INDEX resource_caches_resource_config_id_version_params_hash_uniq`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE resource_caches ALTER COLUMN version TYPE text USING version::text`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_key ON resource_caches (resource_config_id, md5(version), params_hash)`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE worker_resource_config_check_sessions ADD COLUMN team_id integer`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`ALTER TABLE ONLY worker_resource_config_check_sessions
    ADD CONSTRAINT worker_resource_config_check_sessions_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`DROP INDEX worker_resource_config_check_sessions_uniq`)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`CREATE UNIQUE INDEX worker_resource_config_check_sessions_uniq
  ON worker_resource_config_check_sessions (resource_config_check_session_id, worker_base_resource_type_id, team_id)`)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}
