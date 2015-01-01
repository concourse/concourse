package resource

import "github.com/concourse/atc"

type inRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Get(source atc.Source, params atc.Params, version atc.Version) VersionedSource {
	vs := &versionedSource{
		container: resource.container,
	}

	vs.Runner = resource.runScript(
		"/opt/resource/in",
		[]string{ResourcesDir},
		inRequest{source, params, version},
		&vs.versionResult,
	)

	return vs
}

//
// func (resource *resource) extractConfig(input turbine.Input) (turbine.Config, error) {
// 	if input.ConfigPath == "" {
// 		return turbine.Config{}, nil
// 	}
//
// 	configPath := path.Join(ResourcesDir, input.Name, input.ConfigPath)
//
// 	configStream, err := resource.container.StreamOut(configPath)
// 	if err != nil {
// 		return turbine.Config{}, err
// 	}
//
// 	reader := tar.NewReader(configStream)
//
// 	_, err = reader.Next()
// 	if err != nil {
// 		if err == io.EOF {
// 			return turbine.Config{}, fmt.Errorf("could not find build config '%s'", input.ConfigPath)
// 		}
//
// 		return turbine.Config{}, err
// 	}
//
// 	var buildConfig turbine.Config
//
// 	err = candiedyaml.NewDecoder(reader).Decode(&buildConfig)
// 	if err != nil {
// 		return turbine.Config{}, fmt.Errorf("invalid build config '%s': %s", input.ConfigPath, err)
// 	}
//
// 	return buildConfig, nil
// }
