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

type InstanceVars map[string]any

func (iv InstanceVars) String() string {
	if len(iv) == 0 {
		return ""
	}

	var parts []string
	for _, kvPair := range iv.sortedKVPairs() {
		// Marshal value to JSON - handle error case for custom marshalers
		rawVal, err := json.Marshal(kvPair.Value)
		if err != nil {
			// Use placeholder for values that can't be marshaled
			parts = append(parts, fmt.Sprintf("%s:<invalid>", kvPair.Ref))
			continue
		}

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

func instanceVarValueRequiresQuoting(v any) bool {
	str, ok := v.(string)
	if !ok {
		return false
	}

	if strings.ContainsAny(str, ",: /") {
		return true
	}

	var target any
	if err := yaml.Unmarshal([]byte(str), &target); err != nil {
		return true
	}
	_, isStringAfterUnmarshal := target.(string)
	return !isStringAfterUnmarshal
}

func unquoteString(s string) string {
	return strings.TrimPrefix(strings.TrimSuffix(s, `"`), `"`)
}

type PipelineRef struct {
	Name         string       `json:"name"`
	InstanceVars InstanceVars `json:"instance_vars,omitempty"`
}

func (ref PipelineRef) String() string {
	if ref.InstanceVars != nil && len(ref.InstanceVars) > 0 {
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
		payload, err := json.Marshal(kvp.Value)
		if err != nil {
			// Skip values that can't be marshaled
			continue
		}
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

		// Parse the reference
		kvp.Ref, err = vars.ParseReference(k)
		if err != nil {
			return nil, fmt.Errorf("invalid var reference '%s': %w", k, err)
		}

		// Parse the value
		valueStr := q.Get(k)
		if err = json.Unmarshal([]byte(valueStr), &kvp.Value); err != nil {
			return nil, fmt.Errorf("invalid JSON for '%s': %w", k, err)
		}

		kvPairs = append(kvPairs, kvp)
	}

	if len(kvPairs) == 0 {
		return nil, nil
	}

	// Sort for deterministic expansion
	sort.Slice(kvPairs, func(i, j int) bool {
		return kvPairs[i].Ref.String() < kvPairs[j].Ref.String()
	})

	expanded := kvPairs.Expand()
	instanceVars, ok := expanded["vars"].(map[string]any)
	if !ok && expanded["vars"] != nil {
		return nil, fmt.Errorf("expanded vars is not a map")
	}

	return instanceVars, nil
}
