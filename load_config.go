package atc

import "encoding/json"

type LoadConfig struct {
	Path string `yaml:"load,omitempty" json:"load,omitempty" mapstructure:"load"`
}

type LoadTaskConfig struct {
	*LoadConfig
	*TaskConfig
}

func (l *LoadTaskConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var loadConfig LoadConfig
	err := unmarshal(&loadConfig)
	if err != nil {
		return err
	}

	if loadConfig.Path != "" {
		l.LoadConfig = &loadConfig
		return nil
	}

	var taskConfig TaskConfig
	err = unmarshal(&taskConfig)
	if err != nil {
		return err
	}

	l.TaskConfig = &taskConfig
	return nil
}

func (l *LoadTaskConfig) MarshalYAML() (interface{}, error) {
	if l.LoadConfig != nil {
		return l.LoadConfig, nil
	}

	if l.TaskConfig != nil {
		return l.TaskConfig, nil
	}

	return nil, nil
}

func (l *LoadTaskConfig) UnmarshalJSON(version []byte) error {
	var loadConfig LoadConfig
	err := json.Unmarshal(version, &loadConfig)
	if err != nil {
		return err
	}

	if loadConfig.Path != "" {
		l.LoadConfig = &loadConfig
		return nil
	}

	var taskConfig TaskConfig
	err = json.Unmarshal(version, &taskConfig)
	if err != nil {
		return err
	}

	l.TaskConfig = &taskConfig
	return nil
}

func (l *LoadTaskConfig) MarshalJSON() ([]byte, error) {
	if l.LoadConfig != nil {
		return json.Marshal(l.LoadConfig)
	}

	if l.TaskConfig != nil {
		return json.Marshal(l.TaskConfig)
	}

	return json.Marshal("")
}
