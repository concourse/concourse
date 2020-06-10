package db

import (
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"go.opentelemetry.io/otel/api/propagators"
)

//go:generate counterfeiter . ResourceConfigVersion

type ResourceConfigVersion interface {
	ID() int
	Version() Version
	Metadata() ResourceConfigMetadataFields
	CheckOrder() int
	ResourceConfigScope() ResourceConfigScope
	SpanContext() propagators.Supplier

	Reload() (bool, error)
}

type ResourceConfigVersions []ResourceConfigVersion

type ResourceConfigMetadataField struct {
	Name  string
	Value string
}

type ResourceConfigMetadataFields []ResourceConfigMetadataField

func NewResourceConfigMetadataFields(atcm []atc.MetadataField) ResourceConfigMetadataFields {
	metadata := make([]ResourceConfigMetadataField, len(atcm))
	for i, md := range atcm {
		metadata[i] = ResourceConfigMetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}

func (rmf ResourceConfigMetadataFields) ToATCMetadata() []atc.MetadataField {
	metadata := make([]atc.MetadataField, len(rmf))
	for i, md := range rmf {
		metadata[i] = atc.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}

type Version map[string]string

type resourceConfigVersion struct {
	id          int
	version     Version
	metadata    ResourceConfigMetadataFields
	checkOrder  int
	spanContext SpanContext

	resourceConfigScope ResourceConfigScope

	conn Conn
}

var resourceConfigVersionQuery = psql.Select(`
	v.id,
	v.version,
	v.metadata,
	v.check_order,
	v.span_context
`).
	From("resource_config_versions v").
	Where(sq.NotEq{
		"v.check_order": 0,
	})

func (r *resourceConfigVersion) ID() int                                { return r.id }
func (r *resourceConfigVersion) Version() Version                       { return r.version }
func (r *resourceConfigVersion) Metadata() ResourceConfigMetadataFields { return r.metadata }
func (r *resourceConfigVersion) CheckOrder() int                        { return r.checkOrder }
func (r *resourceConfigVersion) ResourceConfigScope() ResourceConfigScope {
	return r.resourceConfigScope
}
func (r *resourceConfigVersion) SpanContext() propagators.Supplier {
	return r.spanContext
}

func (r *resourceConfigVersion) Reload() (bool, error) {
	row := resourceConfigVersionQuery.Where(sq.Eq{"v.id": r.id}).
		RunWith(r.conn).
		QueryRow()

	err := scanResourceConfigVersion(r, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func scanResourceConfigVersion(r *resourceConfigVersion, scan scannable) error {
	var version, metadata, spanContext sql.NullString

	err := scan.Scan(&r.id, &version, &metadata, &r.checkOrder, &spanContext)
	if err != nil {
		return err
	}

	if version.Valid {
		err = json.Unmarshal([]byte(version.String), &r.version)
		if err != nil {
			return err
		}
	}

	if metadata.Valid {
		err = json.Unmarshal([]byte(metadata.String), &r.metadata)
		if err != nil {
			return err
		}
	}

	if spanContext.Valid {
		err = json.Unmarshal([]byte(spanContext.String), &r.spanContext)
		if err != nil {
			return err
		}
	}

	return nil
}
