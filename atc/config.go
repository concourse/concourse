package atc

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/vars"
)

const ConfigVersionHeader = "X-Concourse-Config-Version"
const DefaultTeamName = "main"

type Config struct {
	Groups        GroupConfigs     `json:"groups,omitempty"`
	VarSources    VarSourceConfigs `json:"var_sources,omitempty"`
	Resources     ResourceConfigs  `json:"resources,omitempty"`
	ResourceTypes ResourceTypes    `json:"resource_types,omitempty"`
	Prototypes    Prototypes       `json:"prototypes,omitempty"`
	Jobs          JobConfigs       `json:"jobs,omitempty"`
	Display       *DisplayConfig   `json:"display,omitempty"`
}

func UnmarshalConfig(payload []byte, config interface{}) error {
	// a 'skeleton' of Config, specifying only the toplevel fields
	type skeletonConfig struct {
		Groups        interface{} `json:"groups,omitempty"`
		VarSources    interface{} `json:"var_sources,omitempty"`
		Resources     interface{} `json:"resources,omitempty"`
		ResourceTypes interface{} `json:"resource_types,omitempty"`
		Prototypes    interface{} `json:"prototypes,omitempty"`
		Jobs          interface{} `json:"jobs,omitempty"`
		Display       interface{} `json:"display,omitempty"`
	}

	var stripped skeletonConfig
	err := yaml.Unmarshal(payload, &stripped)
	if err != nil {
		return err
	}

	strippedPayload, err := yaml.Marshal(stripped)
	if err != nil {
		return err
	}

	return yaml.UnmarshalStrict(
		strippedPayload,
		&config,
	)
}

type GroupConfig struct {
	Name      string   `json:"name"`
	Jobs      []string `json:"jobs,omitempty"`
	Resources []string `json:"resources,omitempty"`
}

type GroupConfigs []GroupConfig

func (groups GroupConfigs) Lookup(name string) (GroupConfig, int, bool) {
	for index, group := range groups {
		if group.Name == name {
			return group, index, true
		}
	}

	return GroupConfig{}, -1, false
}

type VarSourceConfig struct {
	Name   string      `json:"name"`
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

type VarSourceConfigs []VarSourceConfig

func (c VarSourceConfigs) Lookup(name string) (VarSourceConfig, bool) {
	for _, cm := range c {
		if cm.Name == name {
			return cm, true
		}
	}

	return VarSourceConfig{}, false
}

type pendingVarSource struct {
	vs   VarSourceConfig
	deps []string
}

func (c VarSourceConfigs) OrderByDependency() (VarSourceConfigs, error) {
	ordered := VarSourceConfigs{}
	pending := []pendingVarSource{}
	added := map[string]interface{}{}

	for _, vs := range c {
		b, err := yaml.Marshal(vs.Config)
		if err != nil {
			return nil, err
		}

		template := vars.NewTemplate(b)
		varNames := template.ExtraVarNames()

		dependencies := []string{}
		for _, varName := range varNames {
			parts := strings.Split(varName, ":")
			if len(parts) > 1 {
				dependencies = append(dependencies, parts[0])
			}
		}

		if len(dependencies) == 0 {
			// If no dependency, add the var source to ordered list.
			ordered = append(ordered, vs)
			added[vs.Name] = true
		} else {
			// If there are some dependencies, then check if dependencies have
			// already been added to ordered list, if yes, then add it; otherwise
			// add it to a pending list.
			miss := false
			for _, dep := range dependencies {
				if added[dep] == nil {
					miss = true
					break
				}
			}
			if !miss {
				ordered = append(ordered, vs)
				added[vs.Name] = true
			} else {
				pending = append(pending, pendingVarSource{vs, dependencies})
				continue
			}
		}

		// Once a var_source is added to ordered list, check if any pending
		// var_source can be added to ordered list.
		left := []pendingVarSource{}
		for _, pendingVs := range pending {
			miss := false
			for _, dep := range pendingVs.deps {
				if added[dep] == nil {
					miss = true
					break
				}
			}
			if !miss {
				ordered = append(ordered, pendingVs.vs)
				added[pendingVs.vs.Name] = true
			} else {
				left = append(left, pendingVs)
			}
		}
		pending = left
	}

	if len(pending) > 0 {
		names := []string{}
		for _, vs := range pending {
			names = append(names, vs.vs.Name)
		}
		return nil, fmt.Errorf("could not resolve inter-dependent var sources: %s", strings.Join(names, ", "))
	}

	return ordered, nil
}

type ResourceConfig struct {
	Name                 string      `json:"name"`
	OldName              string      `json:"old_name,omitempty"`
	Public               bool        `json:"public,omitempty"`
	WebhookToken         string      `json:"webhook_token,omitempty"`
	Type                 string      `json:"type"`
	Source               Source      `json:"source"`
	CheckEvery           *CheckEvery `json:"check_every,omitempty"`
	CheckTimeout         string      `json:"check_timeout,omitempty"`
	Tags                 Tags        `json:"tags,omitempty"`
	Version              Version     `json:"version,omitempty"`
	Icon                 string      `json:"icon,omitempty"`
	ExposeBuildCreatedBy bool        `json:"expose_build_created_by,omitempty"`
}

type ResourceType struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Source     Source      `json:"source"`
	Defaults   Source      `json:"defaults,omitempty"`
	Privileged bool        `json:"privileged,omitempty"`
	CheckEvery *CheckEvery `json:"check_every,omitempty"`
	Tags       Tags        `json:"tags,omitempty"`
	Params     Params      `json:"params,omitempty"`
}

type Prototype struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Source     Source      `json:"source"`
	Defaults   Source      `json:"defaults,omitempty"`
	Privileged bool        `json:"privileged,omitempty"`
	CheckEvery *CheckEvery `json:"check_every,omitempty"`
	Tags       Tags        `json:"tags,omitempty"`
	Params     Params      `json:"params,omitempty"`
}

type DisplayConfig struct {
	BackgroundImage string `json:"background_image,omitempty"`
}

type CheckEvery struct {
	Never    bool
	Interval time.Duration
}

func (c *CheckEvery) UnmarshalJSON(checkEvery []byte) error {
	var data interface{}

	err := json.Unmarshal(checkEvery, &data)
	if err != nil {
		return err
	}

	actual, ok := data.(string)
	if !ok {
		return errors.New("non-string value provided")
	}

	if actual != "" {
		if actual == "never" {
			c.Never = true
			return nil
		}
		c.Interval, err = time.ParseDuration(actual)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CheckEvery) MarshalJSON() ([]byte, error) {
	if c.Never {
		return json.Marshal("never")
	}

	if c.Interval != 0 {
		return json.Marshal(c.Interval.String())
	}

	return json.Marshal("")
}

type Prototypes []Prototype

func (types Prototypes) Lookup(name string) (Prototype, bool) {
	for _, t := range types {
		if t.Name == name {
			return t, true
		}
	}

	return Prototype{}, false
}

type ResourceTypes []ResourceType

func (types ResourceTypes) Lookup(name string) (ResourceType, bool) {
	for _, t := range types {
		if t.Name == name {
			return t, true
		}
	}

	return ResourceType{}, false
}

func (types ResourceTypes) Without(name string) ResourceTypes {
	newTypes := ResourceTypes{}
	for _, t := range types {
		if t.Name != name {
			newTypes = append(newTypes, t)
		}
	}

	return newTypes
}

type ImagePlanner interface {
	ImageForType(planID PlanID, resourceType string, stepTags Tags, skipInterval bool) TypeImage
}

func (types ResourceTypes) ImageForType(planID PlanID, resourceType string, stepTags Tags, skipInterval bool) TypeImage {
	// Check if resource type is a custom type
	parent, found := types.Lookup(resourceType)
	if !found {
		// If it is not a custom type, return back the image as a base type
		return TypeImage{
			BaseType: resourceType,
		}
	}

	imageResource := ImageResource{
		Name:   parent.Name,
		Type:   parent.Type,
		Source: parent.Source,
		Params: parent.Params,
		Tags:   parent.Tags,
	}

	getPlan, checkPlan := FetchImagePlan(planID, imageResource, types.Without(parent.Name), stepTags, skipInterval, parent.CheckEvery)
	checkPlan.Check.ResourceType = resourceType

	return TypeImage{
		// Set the base type as the base type of its parent. The value of the base
		// type will always be the base type at the bottom of the dependency chain.
		//
		// For example, if there is a resource that depends on a custom type that
		// depends on a git base resource type, the BaseType value of the resource's
		// TypeImage will be git.
		BaseType: getPlan.Get.TypeImage.BaseType,

		Privileged: parent.Privileged,

		// GetPlan for fetching the custom type's image and CheckPlan
		// for checking the version of the custom type.
		GetPlan:   &getPlan,
		CheckPlan: checkPlan,
	}
}

func FetchImagePlan(planID PlanID, image ImageResource, resourceTypes ResourceTypes, stepTags Tags, skipInterval bool, checkEvery *CheckEvery) (Plan, *Plan) {
	// If resource type is a custom type, recurse in order to resolve nested resource types
	getPlanID := planID + "/image-get"

	tags := image.Tags
	if len(image.Tags) == 0 {
		tags = stepTags
	}

	// Construct get plan for image
	imageGetPlan := Plan{
		ID: getPlanID,
		Get: &GetPlan{
			Name:   image.Name,
			Type:   image.Type,
			Source: image.Source,
			Params: image.Params,

			TypeImage: resourceTypes.ImageForType(getPlanID, image.Type, tags, skipInterval),

			Tags: tags,
		},
	}

	var maybeCheckPlan *Plan
	if image.Version == nil {
		checkPlanID := planID + "/image-check"
		// don't know the version, need to do a Check before the Get
		interval := CheckEvery{
			Interval: DefaultCheckInterval,
		}

		if checkEvery != nil {
			interval = *checkEvery
		}

		checkPlan := Plan{
			ID: checkPlanID,
			Check: &CheckPlan{
				Name:     image.Name,
				Type:     image.Type,
				Source:   image.Source,
				Interval: interval,

				TypeImage: resourceTypes.ImageForType(checkPlanID, image.Type, tags, skipInterval),

				Tags: tags,

				SkipInterval: skipInterval,
			},
		}
		maybeCheckPlan = &checkPlan

		imageGetPlan.Get.VersionFrom = &checkPlan.ID
	} else {
		// version is already provided, only need to do Get step
		imageGetPlan.Get.Version = &image.Version
	}

	return imageGetPlan, maybeCheckPlan
}

type ResourceConfigs []ResourceConfig

func (resources ResourceConfigs) Lookup(name string) (ResourceConfig, bool) {
	for _, resource := range resources {
		if resource.Name == name {
			return resource, true
		}
	}

	return ResourceConfig{}, false
}

type JobConfigs []JobConfig

func (jobs JobConfigs) Lookup(name string) (JobConfig, bool) {
	for _, job := range jobs {
		if job.Name == name {
			return job, true
		}
	}

	return JobConfig{}, false
}

func (config Config) JobIsPublic(jobName string) (bool, error) {
	job, found := config.Jobs.Lookup(jobName)
	if !found {
		return false, fmt.Errorf("cannot find job with job name '%s'", jobName)
	}

	return job.Public, nil
}

func DefaultTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,

		// https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.CurveP384,
			tls.CurveP521,
		},

		// Security team recommends a very restricted set of cipher suites
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},

		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}
}

func DefaultSSHConfig() ssh.Config {
	return ssh.Config{
		// use the defaults prefered by go, see https://github.com/golang/crypto/blob/master/ssh/common.go
		Ciphers: nil,

		// CIS recommends a certain set of MAC algorithms to be used in SSH connections. This restricts the set from a more permissive set used by default by Go.
		// See https://infosec.mozilla.org/guidelines/openssh.html and https://www.cisecurity.org/cis-benchmarks/
		MACs: []string{
			"hmac-sha2-256-etm@openssh.com",
			"hmac-sha2-256",
		},

		//[KEX Recommendations for SSH IETF](https://tools.ietf.org/html/draft-ietf-curdle-ssh-kex-sha2-10#section-4)
		//[Mozilla Openssh Reference](https://infosec.mozilla.org/guidelines/openssh.html)
		KeyExchanges: []string{
			"ecdh-sha2-nistp256",
			"ecdh-sha2-nistp384",
			"ecdh-sha2-nistp521",
			"curve25519-sha256@libssh.org",
		},
	}
}
