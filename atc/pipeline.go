package atc

import (
	"encoding/json"
	"fmt"
	"net/url"
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
		instanceVarsPayload, _ := json.Marshal(ref.InstanceVars) // TODO to comma separated key:value list
		return fmt.Sprintf("%s: %s", ref.Name, instanceVarsPayload)
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
