package dbng

import (
	"encoding/json"

	"github.com/concourse/atc"
)

type Resource interface {
	ID() int
	Name() string
	Type() string
	Source() atc.Source
}

var resourcesQuery = psql.Select("id, name, config").
	From("resources")

type resource struct {
	id     int
	name   string
	type_  string
	source atc.Source

	conn Conn
}

func (r *resource) ID() int            { return r.id }
func (r *resource) Name() string       { return r.name }
func (r *resource) Type() string       { return r.type_ }
func (r *resource) Source() atc.Source { return r.source }

func scanResource(r *resource, row scannable) error {
	var (
		configBlob []byte
	)

	err := row.Scan(&r.id, &r.name, &configBlob)
	if err != nil {
		return err
	}

	var config atc.Source
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return err
	}

	r.type_ = config.Type
	r.source = config.Source

	return nil
}
