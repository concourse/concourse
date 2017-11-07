package bitbucket

type ProjectConfig struct {
	Name string `json:"project_name,omitempty"`
}

func (flag *ProjectConfig) UnmarshalFlag(value string) error {
	flag.Name = value

	return nil
}
