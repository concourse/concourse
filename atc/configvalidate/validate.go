package configvalidate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/concourse/atc"
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

	groupsWarnings, groupsErr := validateGroups(c)
	if groupsErr != nil {
		errorMessages = append(errorMessages, formatErr("groups", groupsErr))
	}
	warnings = append(warnings, groupsWarnings...)

	resourcesWarnings, resourcesErr := validateResources(c)
	if resourcesErr != nil {
		errorMessages = append(errorMessages, formatErr("resources", resourcesErr))
	}
	warnings = append(warnings, resourcesWarnings...)

	resourceTypesWarnings, resourceTypesErr := validateResourceTypes(c)
	if resourceTypesErr != nil {
		errorMessages = append(errorMessages, formatErr("resource types", resourceTypesErr))
	}
	warnings = append(warnings, resourceTypesWarnings...)

	varSourcesWarnings, varSourcesErr := validateVarSources(c)
	if varSourcesErr != nil {
		errorMessages = append(errorMessages, formatErr("variable sources", varSourcesErr))
	}
	warnings = append(warnings, varSourcesWarnings...)

	jobWarnings, jobsErr := validateJobs(c)
	if jobsErr != nil {
		errorMessages = append(errorMessages, formatErr("jobs", jobsErr))
	}
	warnings = append(warnings, jobWarnings...)

	return warnings, errorMessages
}

func validateGroups(c Config) ([]ConfigWarning, error) {
	var warnings []ConfigWarning
	var errorMessages []string

	jobsGrouped := make(map[string]bool)
	groupNames := make(map[string]int)

	for _, job := range c.Jobs {
		jobsGrouped[job.Name] = false
	}

	for i, group := range c.Groups {
		var identifier string
		if group.Name == "" {
			identifier = fmt.Sprintf("groups[%d]", i)
		} else {
			identifier = fmt.Sprintf("groups.%s", group.Name)
		}

		warning := ValidateIdentifier(group.Name, identifier)
		if warning != nil {
			warnings = append(warnings, *warning)
		}

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

	return warnings, compositeErr(errorMessages)
}

func validateResources(c Config) ([]ConfigWarning, error) {
	var warnings []ConfigWarning
	var errorMessages []string

	names := map[string]int{}

	for i, resource := range c.Resources {
		var identifier string
		if resource.Name == "" {
			identifier = fmt.Sprintf("resources[%d]", i)
		} else {
			identifier = fmt.Sprintf("resources.%s", resource.Name)
		}

		warning := ValidateIdentifier(resource.Name, identifier)
		if warning != nil {
			warnings = append(warnings, *warning)
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

	errorMessages = append(errorMessages, validateResourcesUnused(c)...)

	return warnings, compositeErr(errorMessages)
}

func validateResourceTypes(c Config) ([]ConfigWarning, error) {
	var warnings []ConfigWarning
	var errorMessages []string

	names := map[string]int{}

	for i, resourceType := range c.ResourceTypes {
		var identifier string
		if resourceType.Name == "" {
			identifier = fmt.Sprintf("resource_types[%d]", i)
		} else {
			identifier = fmt.Sprintf("resource_types.%s", resourceType.Name)
		}

		warning := ValidateIdentifier(resourceType.Name, identifier)
		if warning != nil {
			warnings = append(warnings, *warning)
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

	return warnings, compositeErr(errorMessages)
}

func validateResourcesUnused(c Config) []string {
	usedResources := usedResources(c)

	var errorMessages []string
	for _, resource := range c.Resources {
		if _, used := usedResources[resource.Name]; !used {
			message := fmt.Sprintf("resource '%s' is not used", resource.Name)
			errorMessages = append(errorMessages, message)
		}
	}

	return errorMessages
}

func usedResources(c Config) map[string]bool {
	usedResources := make(map[string]bool)

	for _, job := range c.Jobs {
		_ = job.StepConfig().Visit(atc.StepRecursor{
			OnGet: func(step *GetStep) error {
				usedResources[step.ResourceName()] = true
				return nil
			},
			OnPut: func(step *PutStep) error {
				usedResources[step.ResourceName()] = true
				return nil
			},
		})
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

		warning := ValidateIdentifier(job.Name, identifier)
		if warning != nil {
			warnings = append(warnings, *warning)
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

		step := job.Step()

		validator := atc.NewStepValidator(c, []string{identifier, ".plan"})

		_ = validator.Validate(step)

		warnings = append(warnings, validator.Warnings...)

		errorMessages = append(errorMessages, validator.Errors...)
	}

	return warnings, compositeErr(errorMessages)
}

func compositeErr(errorMessages []string) error {
	if len(errorMessages) == 0 {
		return nil
	}

	return errors.New(strings.Join(errorMessages, "\n"))
}

func validateVarSources(c Config) ([]ConfigWarning, error) {
	var warnings []ConfigWarning
	var errorMessages []string

	names := map[string]interface{}{}

	for i, cm := range c.VarSources {
		var identifier string
		if cm.Name == "" {
			identifier = fmt.Sprintf("var_sources[%d]", i)
		} else {
			identifier = fmt.Sprintf("var_sources.%s", cm.Name)
		}

		warning := ValidateIdentifier(cm.Name, identifier)
		if warning != nil {
			warnings = append(warnings, *warning)
		}

		if factory, exists := creds.ManagerFactories()[cm.Type]; exists {
			// TODO: this check should eventually be removed once all credential managers
			// are supported in pipeline. - @evanchaoli
			switch cm.Type {
			case "vault", "dummy", "ssm":
			default:
				errorMessages = append(errorMessages, fmt.Sprintf("credential manager type %s is not supported in pipeline yet", cm.Type))
			}

			if _, ok := names[cm.Name]; ok {
				errorMessages = append(errorMessages, fmt.Sprintf("duplicate var_source name: %s", cm.Name))
			}
			names[cm.Name] = 0

			if manager, err := factory.NewInstance(cm.Config); err == nil {
				err = manager.Validate()
				if err != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("credential manager %s is invalid: %s", cm.Name, err.Error()))
				}
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to create credential manager %s: %s", cm.Name, err.Error()))
			}
		} else {
			errorMessages = append(errorMessages, fmt.Sprintf("unknown credential manager type: %s", cm.Type))
		}
	}

	if _, err := c.VarSources.OrderByDependency(); err != nil {
		errorMessages = append(errorMessages, fmt.Sprintf("failed to order by dependency: %s", err.Error()))
	}

	return warnings, compositeErr(errorMessages)
}
