package configvalidate

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	. "github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
)

func formatErr(groupName string, err error) string {
	lines := strings.Split(err.Error(), "\n")
	indented := make([]string, len(lines))

	for i, l := range lines {
		indented[i] = "\t" + l
	}

	return fmt.Sprintf("invalid %s:\n%s\n", groupName, strings.Join(indented, "\n"))
}

func Validate(c Config) ([]ConfigWarning, []string) {
	warnings := []ConfigWarning{}
	errorMessages := []string{}

	groupsErr := validateGroups(c)
	if groupsErr != nil {
		errorMessages = append(errorMessages, formatErr("groups", groupsErr))
	}

	resourcesErr := validateResources(c)
	if resourcesErr != nil {
		errorMessages = append(errorMessages, formatErr("resources", resourcesErr))
	}

	resourceTypesWarnings, resourceTypesErr := validateResourceTypes(c)
	if resourceTypesErr != nil {
		errorMessages = append(errorMessages, formatErr("resource types", resourceTypesErr))
	}
	warnings = append(warnings, resourceTypesWarnings...)

	varSourcesErr := validateVarSources(c)
	if varSourcesErr != nil {
		errorMessages = append(errorMessages, formatErr("variable sources", varSourcesErr))
	}

	jobWarnings, jobsErr := validateJobs(c)
	if jobsErr != nil {
		errorMessages = append(errorMessages, formatErr("jobs", jobsErr))
	}
	warnings = append(warnings, jobWarnings...)

	return warnings, errorMessages
}

func validateGroups(c Config) error {
	var errorMessages []string

	jobsGrouped := make(map[string]bool)
	groupNames := make(map[string]int)

	for _, job := range c.Jobs {
		jobsGrouped[job.Name] = false
	}

	for _, group := range c.Groups {

		if val, ok := groupNames[group.Name]; ok {
			groupNames[group.Name] = val + 1

		} else {
			groupNames[group.Name] = 1
		}

		for _, job := range group.Jobs {
			_, exists := c.Jobs.Lookup(job)
			if !exists {
				errorMessages = append(errorMessages,
					fmt.Sprintf("group '%s' has unknown job '%s'", group.Name, job))
			} else {
				jobsGrouped[job] = true
			}
		}

		for _, resource := range group.Resources {
			_, exists := c.Resources.Lookup(resource)
			if !exists {
				errorMessages = append(errorMessages,
					fmt.Sprintf("group '%s' has unknown resource '%s'", group.Name, resource))
			}
		}
	}

	for groupName, groupCount := range groupNames {
		if groupCount > 1 {
			errorMessages = append(errorMessages,
				fmt.Sprintf("group '%s' appears %d times. Duplicate names are not allowed.", groupName, groupCount))
		}
	}

	if len(c.Groups) != 0 {
		for job, grouped := range jobsGrouped {
			if !grouped {
				errorMessages = append(errorMessages, fmt.Sprintf("job '%s' belongs to no group", job))
			}
		}
	}

	return compositeErr(errorMessages)
}

func validateResources(c Config) error {
	var errorMessages []string

	names := map[string]int{}

	for i, resource := range c.Resources {
		var identifier string
		if resource.Name == "" {
			identifier = fmt.Sprintf("resources[%d]", i)
		} else {
			identifier = fmt.Sprintf("resources.%s", resource.Name)
		}

		if other, exists := names[resource.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"resources[%d] and resources[%d] have the same name ('%s')",
					other, i, resource.Name))
		} else if resource.Name != "" {
			names[resource.Name] = i
		}

		if resource.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if resource.Type == "" {
			errorMessages = append(errorMessages, identifier+" has no type")
		}
	}

	return compositeErr(errorMessages)
}

func validateResourceTypes(c Config) ([]ConfigWarning, error) {
	errorMessages := []string{}
	warnings := []ConfigWarning{}

	names := map[string]int{}

	for i, resourceType := range c.ResourceTypes {
		var identifier string
		if resourceType.Name == "" {
			identifier = fmt.Sprintf("resource_types[%d]", i)
		} else {
			identifier = fmt.Sprintf("resource_types.%s", resourceType.Name)
		}

		if other, exists := names[resourceType.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"resource_types[%d] and resource_types[%d] have the same name ('%s')",
					other, i, resourceType.Name))
		} else if resourceType.Name != "" {
			names[resourceType.Name] = i
		}

		if resourceType.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if resourceType.Type == "" {
			errorMessages = append(errorMessages, identifier+" has no type")
		}
	}

	warnings = append(warnings, validateResourcesUnused(c)...)

	return warnings, compositeErr(errorMessages)
}

func validateResourcesUnused(c Config) []ConfigWarning {
	warnings := []ConfigWarning{}
	usedResources := usedResources(c)

	for _, resource := range c.Resources {
		if _, used := usedResources[resource.Name]; !used {
			warnings = append(warnings, ConfigWarning{
				Type:    "resources",
				Message: resource.Name + " : is not used in pipeline",
			})
		}
	}

	return warnings
}

func usedResources(c Config) map[string]bool {
	usedResources := make(map[string]bool)

	for _, job := range c.Jobs {
		for _, input := range job.Inputs() {
			usedResources[input.Resource] = true
		}
		for _, output := range job.Outputs() {
			usedResources[output.Resource] = true
		}
	}

	return usedResources
}

func validateJobs(c Config) ([]ConfigWarning, error) {
	var errorMessages []string
	var warnings []ConfigWarning

	names := map[string]int{}

	for i, job := range c.Jobs {
		var identifier string
		if job.Name == "" {
			identifier = fmt.Sprintf("jobs[%d]", i)
		} else {
			identifier = fmt.Sprintf("jobs.%s", job.Name)
		}

		if other, exists := names[job.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"jobs[%d] and jobs[%d] have the same name ('%s')",
					other, i, job.Name))
		} else if job.Name != "" {
			names[job.Name] = i
		}

		if job.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if job.BuildLogRetention != nil && job.BuildLogsToRetain != 0 {
			errorMessages = append(
				errorMessages,
				identifier+fmt.Sprintf(" can't use both build_log_retention and build_logs_to_retain"),
			)
		} else if job.BuildLogsToRetain < 0 {
			errorMessages = append(
				errorMessages,
				identifier+fmt.Sprintf(" has negative build_logs_to_retain: %d", job.BuildLogsToRetain),
			)
		}

		if job.BuildLogRetention != nil {
			if job.BuildLogRetention.Builds < 0 {
				errorMessages = append(
					errorMessages,
					identifier+fmt.Sprintf(" has negative build_log_retention.builds: %d", job.BuildLogRetention.Builds),
				)
			}
			if job.BuildLogRetention.Days < 0 {
				errorMessages = append(
					errorMessages,
					identifier+fmt.Sprintf(" has negative build_log_retention.days: %d", job.BuildLogRetention.Days),
				)
			}
			if job.BuildLogRetention.MinimumSucceededBuilds < 0 {
				errorMessages = append(
					errorMessages,
					identifier+fmt.Sprintf(" has negative build_log_retention.min_success_builds: %d", job.BuildLogRetention.MinimumSucceededBuilds),
				)
			}
			if job.BuildLogRetention.Builds > 0 && job.BuildLogRetention.MinimumSucceededBuilds > job.BuildLogRetention.Builds {
				errorMessages = append(
					errorMessages,
					identifier+fmt.Sprintf(" has build_log_retention.min_success_builds: %d greater than build_log_retention.min_success_builds: %d", job.BuildLogRetention.MinimumSucceededBuilds, job.BuildLogRetention.Builds),
				)
			}
		}

		planWarnings, planErrMessages := validatePlan(c, identifier+".plan", PlanConfig{Do: &job.Plan})
		warnings = append(warnings, planWarnings...)
		errorMessages = append(errorMessages, planErrMessages...)

		if job.Abort != nil {
			subIdentifier := fmt.Sprintf("%s.abort", identifier)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, *job.Abort)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

		if job.Error != nil {
			subIdentifier := fmt.Sprintf("%s.error", identifier)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, *job.Error)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

		if job.Failure != nil {
			subIdentifier := fmt.Sprintf("%s.failure", identifier)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, *job.Failure)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

		if job.Ensure != nil {
			subIdentifier := fmt.Sprintf("%s.ensure", identifier)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, *job.Ensure)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

		if job.Success != nil {
			subIdentifier := fmt.Sprintf("%s.success", identifier)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, *job.Success)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

		encountered := map[string]int{}
		for _, input := range job.Inputs() {
			encountered[input.Name]++

			if encountered[input.Name] == 2 {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf("%s has get steps with the same name: %s", identifier, input.Name),
				)
			}
		}
	}

	return warnings, compositeErr(errorMessages)
}

type foundTypes struct {
	identifier string
	found      map[string]bool
}

func (ft *foundTypes) Find(name string) {
	ft.found[name] = true
}

func (ft foundTypes) IsValid() (bool, string) {
	if len(ft.found) == 0 {
		return false, ft.identifier + " has no action specified"
	}

	if len(ft.found) > 1 {
		types := make([]string, 0, len(ft.found))

		for typee := range ft.found {
			types = append(types, typee)
		}

		sort.Strings(types)

		return false, fmt.Sprintf("%s has multiple actions specified (%s)", ft.identifier, strings.Join(types, ", "))
	}

	return true, ""
}

func validatePlan(c Config, identifier string, plan PlanConfig) ([]ConfigWarning, []string) {
	foundTypes := foundTypes{
		identifier: identifier,
		found:      make(map[string]bool),
	}

	if plan.Get != "" {
		foundTypes.Find("get")
	}

	if plan.Put != "" {
		foundTypes.Find("put")
	}

	if plan.Task != "" {
		foundTypes.Find("task")
	}

	if plan.SetPipeline != "" {
		foundTypes.Find("set_pipeline")
	}

	if plan.Do != nil {
		foundTypes.Find("do")
	}

	if plan.Aggregate != nil {
		foundTypes.Find("aggregate")
	}

	if plan.InParallel != nil {
		foundTypes.Find("parallel")
	}

	if plan.Try != nil {
		foundTypes.Find("try")
	}

	if valid, message := foundTypes.IsValid(); !valid {
		return []ConfigWarning{}, []string{message}
	}

	var errorMessages []string
	var warnings []ConfigWarning

	switch {
	case plan.Do != nil:
		for i, plan := range *plan.Do {
			subIdentifier := fmt.Sprintf("%s[%d]", identifier, i)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, plan)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

	case plan.Aggregate != nil:
		warnings = append(warnings, ConfigWarning{
			Type:    "pipeline",
			Message: identifier + " : aggregate is deprecated and will be removed in a future version",
		})
		for i, plan := range *plan.Aggregate {
			subIdentifier := fmt.Sprintf("%s.aggregate[%d]", identifier, i)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, plan)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

	case plan.InParallel != nil:
		for i, plan := range plan.InParallel.Steps {
			subIdentifier := fmt.Sprintf("%s.in_parallel[%d]", identifier, i)
			planWarnings, planErrMessages := validatePlan(c, subIdentifier, plan)
			warnings = append(warnings, planWarnings...)
			errorMessages = append(errorMessages, planErrMessages...)
		}

	case plan.Get != "":
		identifier = fmt.Sprintf("%s.get.%s", identifier, plan.Get)

		errorMessages = append(errorMessages, validateInapplicableFields(
			[]string{"privileged", "config", "file"},
			plan, identifier)...,
		)

		if plan.Resource != "" {
			_, found := c.Resources.Lookup(plan.Resource)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist ('%s')",
						identifier,
						plan.Resource,
					),
				)
			}
		} else {
			_, found := c.Resources.Lookup(plan.Get)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist",
						identifier,
					),
				)
			}
		}

		for _, job := range plan.Passed {
			jobConfig, found := c.Jobs.Lookup(job)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s.passed references an unknown job ('%s')",
						identifier,
						job,
					),
				)
			} else {
				foundResource := false

				for _, input := range jobConfig.Inputs() {
					if input.Resource == plan.ResourceName() {
						foundResource = true
						break
					}
				}

				for _, output := range jobConfig.Outputs() {
					if output.Resource == plan.ResourceName() {
						foundResource = true
						break
					}
				}

				if !foundResource {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"%s.passed references a job ('%s') which doesn't interact with the resource ('%s')",
							identifier,
							job,
							plan.Get,
						),
					)
				}
			}
		}

	case plan.Put != "":
		identifier = fmt.Sprintf("%s.put.%s", identifier, plan.Put)

		errorMessages = append(errorMessages, validateInapplicableFields(
			[]string{"passed", "trigger", "privileged", "config", "file"},
			plan, identifier)...,
		)

		if plan.Resource != "" {
			_, found := c.Resources.Lookup(plan.Resource)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist ('%s')",
						identifier,
						plan.Resource,
					),
				)
			}
		} else {
			_, found := c.Resources.Lookup(plan.Put)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist",
						identifier,
					),
				)
			}
		}

	case plan.Task != "":
		identifier = fmt.Sprintf("%s.task.%s", identifier, plan.Task)

		if plan.TaskConfig == nil && plan.ConfigPath == "" {
			errorMessages = append(errorMessages, identifier+" does not specify any task configuration")
		}

		if plan.TaskConfig != nil && (plan.TaskConfig.RootfsURI != "" || plan.TaskConfig.ImageResource != nil) && plan.ImageArtifactName != "" {
			warnings = append(warnings, ConfigWarning{
				Type:    "pipeline",
				Message: identifier + " specifies an image artifact to use as the container's image but also specifies an image or image resource in the task configuration; the image artifact takes precedence",
			})
		}

		if plan.TaskConfig != nil && plan.ConfigPath != "" {
			errorMessages = append(errorMessages, identifier+" specifies both `file` and `config` in a task step")
		}

		if plan.TaskConfig != nil {
			if err := plan.TaskConfig.Validate(); err != nil {
				messages := strings.Split(err.Error(), "\n")
				for _, message := range messages {
					errorMessages = append(errorMessages, fmt.Sprintf("%s %s", identifier, strings.TrimSpace(message)))
				}
			}
		}

		errorMessages = append(errorMessages, validateInapplicableFields(
			[]string{"resource", "passed", "trigger"},
			plan, identifier)...,
		)

	case plan.SetPipeline != "":
		identifier = fmt.Sprintf("%s.set_pipeline.%s", identifier, plan.SetPipeline)

		if plan.ConfigPath == "" {
			errorMessages = append(errorMessages, identifier+" does not specify any pipeline configuration")
		}

	case plan.Try != nil:
		subIdentifier := fmt.Sprintf("%s.try", identifier)
		planWarnings, planErrMessages := validatePlan(c, subIdentifier, *plan.Try)
		warnings = append(warnings, planWarnings...)
		errorMessages = append(errorMessages, planErrMessages...)
	}

	if plan.Abort != nil {
		subIdentifier := fmt.Sprintf("%s.abort", identifier)
		planWarnings, planErrMessages := validatePlan(c, subIdentifier, *plan.Abort)
		warnings = append(warnings, planWarnings...)
		errorMessages = append(errorMessages, planErrMessages...)
	}

	if plan.Error != nil {
		subIdentifier := fmt.Sprintf("%s.error", identifier)
		planWarnings, planErrMessages := validatePlan(c, subIdentifier, *plan.Error)
		warnings = append(warnings, planWarnings...)
		errorMessages = append(errorMessages, planErrMessages...)
	}

	if plan.Ensure != nil {
		subIdentifier := fmt.Sprintf("%s.ensure", identifier)
		planWarnings, planErrMessages := validatePlan(c, subIdentifier, *plan.Ensure)
		warnings = append(warnings, planWarnings...)
		errorMessages = append(errorMessages, planErrMessages...)
	}

	if plan.Success != nil {
		subIdentifier := fmt.Sprintf("%s.success", identifier)
		planWarnings, planErrMessages := validatePlan(c, subIdentifier, *plan.Success)
		warnings = append(warnings, planWarnings...)
		errorMessages = append(errorMessages, planErrMessages...)
	}

	if plan.Failure != nil {
		subIdentifier := fmt.Sprintf("%s.failure", identifier)
		planWarnings, planErrMessages := validatePlan(c, subIdentifier, *plan.Failure)
		warnings = append(warnings, planWarnings...)
		errorMessages = append(errorMessages, planErrMessages...)
	}

	if plan.Timeout != "" {
		_, err := time.ParseDuration(plan.Timeout)
		if err != nil {
			subIdentifier := fmt.Sprintf("%s.timeout", identifier)
			errorMessages = append(errorMessages, subIdentifier+fmt.Sprintf(" refers to a duration that could not be parsed ('%s')", plan.Timeout))
		}
	}

	if plan.Attempts < 0 {
		subIdentifier := fmt.Sprintf("%s.attempts", identifier)
		errorMessages = append(errorMessages, subIdentifier+fmt.Sprintf(" has an invalid number of attempts (%d)", plan.Attempts))
	}

	return warnings, errorMessages
}

func validateInapplicableFields(inapplicableFields []string, plan PlanConfig, identifier string) []string {
	var errorMessages []string
	var foundInapplicableFields []string

	for _, field := range inapplicableFields {
		switch field {
		case "resource":
			if plan.Resource != "" {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "passed":
			if len(plan.Passed) != 0 {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "trigger":
			if plan.Trigger {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "privileged":
			if plan.Privileged {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "config":
			if plan.TaskConfig != nil {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "file":
			if plan.ConfigPath != "" {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		}
	}

	if len(foundInapplicableFields) > 0 {
		errorMessages = append(
			errorMessages,
			fmt.Sprintf(
				"%s has invalid fields specified (%s)",
				identifier,
				strings.Join(foundInapplicableFields, ", "),
			),
		)
	}

	return errorMessages
}

func compositeErr(errorMessages []string) error {
	if len(errorMessages) == 0 {
		return nil
	}

	return errors.New(strings.Join(errorMessages, "\n"))
}

func validateVarSources(c Config) error {
	names := map[string]interface{}{}

	for _, cm := range c.VarSources {
		factory := creds.ManagerFactories()[cm.Type]
		if factory == nil {
			return fmt.Errorf("unknown credential manager type: %s", cm.Type)
		}

		// TODO: this check should eventually be removed once all credential managers
		// are supported in pipeline. - @evanchaoli
		switch cm.Type {
		case "vault", "dummy":
		default:
			return fmt.Errorf("credential manager type %s is not supported in pipeline yet", cm.Type)
		}

		if _, ok := names[cm.Name]; ok {
			return fmt.Errorf("duplicate var_source name: %s", cm.Name)
		}
		names[cm.Name] = 0

		manager, err := factory.NewInstance(cm.Config)
		if err != nil {
			return fmt.Errorf("failed to create credential manager %s: %s", cm.Name, err.Error())
		}
		err = manager.Validate()
		if err != nil {
			return fmt.Errorf("credential manager %s is invalid: %s", cm.Name, err.Error())
		}
	}

	if _, err := c.VarSources.OrderByDependency(); err != nil {
		return err
	}

	return nil
}
