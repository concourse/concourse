package atc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/concourse/concourse/vars"
	"sigs.k8s.io/yaml"
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

func (iv InstanceVars) String() string {
	kvPairs := vars.StaticVariables(iv).Flatten()
	sort.Slice(kvPairs, func(i, j int) bool {
		return kvPairs[i].Ref.String() < kvPairs[j].Ref.String()
	})

	var parts []string
	for _, kvPair := range kvPairs {
		rawVal, _ := json.Marshal(kvPair.Value)
		val := string(rawVal)
		if !requiresQuoting(kvPair.Value) {
			val = unquoteString(val)
		}
		parts = append(parts, fmt.Sprintf("%s:%s", kvPair.Ref, val))
	}
	return strings.Join(parts, ",")
}

func requiresQuoting(v interface{}) bool {
	str, ok := v.(string)
	if !ok {
		return false
	}
	var target interface{}
	if err := yaml.Unmarshal([]byte(str), &target); err != nil {
		return true
	}
	_, isStringAfterUnmarshal := target.(string)
	if !isStringAfterUnmarshal {
		return true
	}
	return strings.ContainsAny(str, ",: ")
}

func unquoteString(s string) string {
	return strings.TrimPrefix(strings.TrimSuffix(s, `"`), `"`)
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
