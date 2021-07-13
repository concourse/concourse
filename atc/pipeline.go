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
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	InstanceVars  InstanceVars   `json:"instance_vars,omitempty"`
	Paused        bool           `json:"paused"`
	PausedBy      string         `json:"paused_by,omitempty"`
	PausedAt      int64          `json:"paused_at,omitempty"`
	Public        bool           `json:"public"`
	Archived      bool           `json:"archived"`
	Groups        GroupConfigs   `json:"groups,omitempty"`
	TeamName      string         `json:"team_name"`
	Display       *DisplayConfig `json:"display,omitempty"`
	ParentBuildID int            `json:"parent_build_id,omitempty"`
	ParentJobID   int            `json:"parent_job_id,omitempty"`
	LastUpdated   int64          `json:"last_updated,omitempty"`
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
	var parts []string
	for _, kvPair := range iv.sortedKVPairs() {
		rawVal, _ := json.Marshal(kvPair.Value)
		val := string(rawVal)
		if !instanceVarValueRequiresQuoting(kvPair.Value) {
			val = unquoteString(val)
		}
		parts = append(parts, fmt.Sprintf("%s:%s", kvPair.Ref, val))
	}
	return strings.Join(parts, ",")
}

func (iv InstanceVars) sortedKVPairs() vars.KVPairs {
	kvPairs := vars.StaticVariables(iv).Flatten()
	sort.Slice(kvPairs, func(i, j int) bool {
		return kvPairs[i].Ref.String() < kvPairs[j].Ref.String()
	})
	return kvPairs
}

func instanceVarValueRequiresQuoting(v interface{}) bool {
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
	return strings.ContainsAny(str, ",: /")
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
	if len(ref.InstanceVars) == 0 {
		return nil
	}
	params := url.Values{}
	for _, kvp := range ref.InstanceVars.sortedKVPairs() {
		payload, _ := json.Marshal(kvp.Value)
		params.Set("vars."+kvp.Ref.String(), string(payload))
	}
	return params
}

func InstanceVarsFromQueryParams(q url.Values) (InstanceVars, error) {
	var kvPairs vars.KVPairs
	for k := range q {
		if k != "vars" && !strings.HasPrefix(k, "vars.") {
			continue
		}
		var kvp vars.KVPair
		var err error
		kvp.Ref, err = vars.ParseReference(k)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(q.Get(k)), &kvp.Value); err != nil {
			return nil, err
		}
		kvPairs = append(kvPairs, kvp)
	}
	if len(kvPairs) == 0 {
		return nil, nil
	}
	// Need to sort kv-pairs so that you can specify ?vars={...} along with
	// patches (...&vars.foo=123) and not have the patches occasionally
	// truncated (due to non-deterministic map iteration)
	sort.Slice(kvPairs, func(i, j int) bool {
		return kvPairs[i].Ref.String() < kvPairs[j].Ref.String()
	})
	instanceVars, _ := kvPairs.Expand()["vars"].(map[string]interface{})
	return InstanceVars(instanceVars), nil
}
