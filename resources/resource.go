package resources

import "github.com/winston-ci/prole/api/builds"

type Resource struct {
	Name string

	Type string
	URI  string
}

func (resource Resource) BuildSource() builds.BuildSource {
	return builds.BuildSource{
		Type:   resource.Type,
		URI:    resource.URI,
		Branch: "master",
		Ref:    "HEAD",
	}
}
