package atc

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/imdario/mergo"
)

type Pipeline struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	InstanceVars InstanceVars   `json:"instance_vars,omitempty"`
	Paused       bool           `json:"paused"`
	Public       bool           `json:"public"`
	Archived     bool           `json:"archived"`
	Groups       GroupConfigs   `json:"groups,omitempty"`
	TeamName     string         `json:"team_name"`
	Display      *DisplayConfig `json:"display,omitempty"`
	LastUpdated  int64          `json:"last_updated,omitempty"`
}

func (p Pipeline) Ref() PipelineRef {
	return PipelineRef{
		Name:         p.Name,
		InstanceVars: p.InstanceVars,
	}
}

type RenameRequest struct {
	NewName string `json:"name"`
}

type InstanceVars map[string]interface{}

func (iv InstanceVars) Flatten() (DotNotation, error) {
	return flatten("", 0, (map[string]interface{})(iv))
}

func (iv InstanceVars) String() string {
	m, _ := iv.Flatten()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var instanceVars []string
	for _, k := range keys {
		bytes, _ := yaml.Marshal(m[k])
		bytes = []byte(strings.TrimRight(string(bytes), "\n"))
		instanceVars = append(instanceVars, fmt.Sprintf("%s:%s", k, bytes))
	}
	return strings.Join(instanceVars, ",")
}

type DotNotation map[string]interface{}

func (d DotNotation) Expand() (InstanceVars, error) {
	var out interface{}
	for key, value := range d {
		if err := expand(&out, strings.Split(key, ".")[:], value); err != nil {
			return nil, err
		}
	}
	return (out).(map[string]interface{}), nil
}

type PipelineRef struct {
	Name         string       `json:"name"`
	InstanceVars InstanceVars `json:"instance_vars,omitempty"`
}

func (ref PipelineRef) String() string {
	if ref.InstanceVars != nil {
		return fmt.Sprintf("%s/%s", ref.Name, ref.InstanceVars.String())
	}
	return ref.Name
}

func (ref PipelineRef) QueryParams() url.Values {
	if ref.InstanceVars != nil {
		payload, _ := json.Marshal(ref.InstanceVars)
		return url.Values{"instance_vars": []string{string(payload)}}
	}
	return nil
}

type OrderPipelinesRequest []PipelineRef

func (r OrderPipelinesRequest) Len() int {
	return len(r)
}

func (r OrderPipelinesRequest) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r OrderPipelinesRequest) Less(i, j int) bool {
	return r[i].String() < r[j].String()
}

func flatten(prefix string, level int, value interface{}) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	switch val := value.(type) {
	case map[string]interface{}:
		for k, v := range val {
			newKey := k
			if prefix != "" {
				newKey = prefix + "." + newKey
			}
			m2, err := flatten(newKey, level+1, v)
			if err != nil {
				return nil, err
			}
			err = mergo.Merge(&m, m2)
			if err != nil {
				return nil, err
			}
		}
	case []interface{}:
		for i, v := range val {
			newKey := strconv.Itoa(i)
			if prefix != "" {
				newKey = prefix + "." + newKey
			}
			m2, err := flatten(newKey, level+1, v)
			if err != nil {
				return nil, err
			}
			err = mergo.Merge(&m, m2)
			if err != nil {
				return nil, err
			}
		}
	default:
		m[prefix] = val
	}
	return m, nil
}

func expand(out *interface{}, keys []string, value interface{}) error {
	if len(keys) > 0 {
		key := keys[0]
		pos, err := strconv.Atoi(key)
		if err != nil {
			err = expandMap(out, key, keys, value)
			if err != nil {
				return err
			}
		} else {
			err = expandSlice(out, pos, keys, value)
			if err != nil {
				return err
			}
		}
	} else {
		*out = value
	}
	return nil
}

func expandMap(out *interface{}, key string, keys []string, value interface{}) error {
	if *out == nil {
		*out = make(map[string]interface{})
	}

	current, ok := (*out).(map[string]interface{})
	if !ok {
		return errors.New("error while asserting type")
	}

	var next = current[key]
	if err := expand(&next, keys[1:], value); err != nil {
		return err
	}

	current[key] = next

	return nil
}

func expandSlice(out *interface{}, pos int, keys []string, value interface{}) error {
	if *out == nil {
		*out = make([]interface{}, 0)
	}

	slice, ok := (*out).([]interface{})
	if !ok {
		return errors.New("error while asserting type")
	}

	if len(slice) < pos+1 {
		for i := 0; i < pos+1; i++ {
			if len((*out).([]interface{})) < i+1 {
				a := (*out).([]interface{})
				*out = append(a, nil)
			}
		}
	}

	var next = (*out).([]interface{})[pos]
	if err := expand(&next, keys[1:], value); err != nil {
		return err
	}

	(*out).([]interface{})[pos] = next

	return nil
}
