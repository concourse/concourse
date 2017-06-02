package migrations

import "github.com/concourse/atc/db/migration"

func AddHttpProxyHttpsProxyNoProxyToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN http_proxy_url text,
		ADD COLUMN https_proxy_url text,
		ADD COLUMN no_proxy text
	`)
	return err
}
