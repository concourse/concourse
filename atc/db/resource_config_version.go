package db

//go:generate counterfeiter . ResourceConfigVersion

type ResourceConfigVersion interface {
	ID() int
	ResourceConfigID() int
	Version() Version
	Metadata() ResourceConfigMetadataFields
	CheckOrder() int
}

type ResourceConfigMetadataField struct {
	Name  string
	Value string
}

type ResourceConfigMetadataFields []ResourceConfigMetadataField
type Version map[string]string

type resourceConfigVersion struct {
	id               int
	resourceConfigID int
	version          Version
	metadata         ResourceConfigMetadataFields
	checkOrder       int
}

func (r *resourceConfigVersion) ID() int                                { return r.id }
func (r *resourceConfigVersion) ResourceConfigID() int                  { return r.resourceConfigID }
func (r *resourceConfigVersion) Version() Version                       { return r.version }
func (r *resourceConfigVersion) Metadata() ResourceConfigMetadataFields { return r.metadata }
func (r *resourceConfigVersion) CheckOrder() int                        { return r.checkOrder }
