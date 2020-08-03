package atc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

type InstanceVars map[string]string

type PipelineRef struct {
	Name         string       `json:"name"`
	InstanceVars InstanceVars `json:"instance_vars,omitempty"`
}

func (ref PipelineRef) String() string {
	if ref.InstanceVars != nil {
		var instanceVars []string
		for k, v := range ref.InstanceVars {
			instanceVars = append(instanceVars, fmt.Sprintf("%s:%s", k, v))
		}
		return fmt.Sprintf("%s/%s", ref.Name, strings.Join(instanceVars, ","))
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
