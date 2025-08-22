package configvalidate

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/gobwas/glob"
)

func formatErr(groupName string, err error) string {
	lines := strings.Split(err.Error(), "\n")
	indented := make([]string, len(lines))

	for i, l := range lines {
		indented[i] = "\t" + l
	}

	return fmt.Sprintf("invalid %s:\n%s\n", groupName, strings.Join(indented, "\n"))
}

type location struct {
	section string
	index   int
}

func (l location) String() string {
	return fmt.Sprintf("%s[%d]", l.section, l.index)
}

func (l location) Identifier(name string) string {
	if name == "" {
		return l.String()
	}
	return fmt.Sprintf("%s.%s", l.section, name)
}

func Validate(c atc.Config) ([]atc.ConfigErrors, []string) {
	configErrors := []atc.ConfigErrors{}
	errorMessages := []string{}

	groupsConfigError, groupsErr := validateGroups(c)
	if groupsErr != nil {
		errorMessages = append(errorMessages, formatErr("groups", groupsErr))
	}

	configErrors = append(configErrors, groupsConfigError...)

	resourcesConfigError, resourcesErr := validateResources(c)
	if resourcesErr != nil {
		errorMessages = append(errorMessages, formatErr("resources", resourcesErr))
	}

	configErrors = append(configErrors, resourcesConfigError...)

	seenTypes := make(map[string]location)

	resourcesTypeConfigError, resourceTypesErr := validateResourceTypes(c, seenTypes)
	if resourceTypesErr != nil {
		errorMessages = append(errorMessages, formatErr("resource types", resourceTypesErr))
	}

	configErrors = append(configErrors, resourcesTypeConfigError...)

	prototypesConfigError, prototypesErr := validatePrototypes(c, seenTypes)
	if prototypesErr != nil {
		errorMessages = append(errorMessages, formatErr("prototypes", prototypesErr))
	}

	configErrors = append(configErrors, prototypesConfigError...)

	varSourcesConfigError, varSourcesErr := validateVarSources(c)
	if varSourcesErr != nil {
		errorMessages = append(errorMessages, formatErr("variable sources", varSourcesErr))
	}

	configErrors = append(configErrors, varSourcesConfigError...)

	jobsConfigError, jobsErr := validateJobs(c)
	if jobsErr != nil {
		errorMessages = append(errorMessages, formatErr("jobs", jobsErr))
	}

	configErrors = append(configErrors, jobsConfigError...)

	displayConfigErrors, displayErr := validateDisplay(c)
	if displayErr != nil {
		errorMessages = append(errorMessages, formatErr("display config", displayErr))
	}

	configErrors = append(configErrors, displayConfigErrors...)

	cycleErr := validateCycle(c)

	if cycleErr != nil {
		errorMessages = append(errorMessages, formatErr("jobs", cycleErr))
	}

	return configErrors, errorMessages
}

func validateGroups(c atc.Config) ([]atc.ConfigErrors, error) {
	var errorMessages []string
	var configErrors []atc.ConfigErrors

	jobsGrouped := make(map[string]bool)
	groupNames := make(map[string]int)

	for _, job := range c.Jobs {
		jobsGrouped[job.Name] = false
	}

	for i, group := range c.Groups {
		location := location{section: "groups", index: i}
		identifier := location.Identifier(group.Name)

		configError := atc.ValidateIdentifier(group.Name, identifier)
		if configError != nil {
			configErrors = append(configErrors, *configError)
		}

		if val, ok := groupNames[group.Name]; ok {
			groupNames[group.Name] = val + 1

		} else {
			groupNames[group.Name] = 1
		}

		for _, jobGlob := range group.Jobs {
			matchingJob := false
			g, err := glob.Compile(jobGlob)
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("invalid glob expression '%s' for group '%s'", jobGlob, group.Name))
				continue
			}
			for _, job := range c.Jobs {
				if g.Match(job.Name) {
					jobsGrouped[job.Name] = true
					matchingJob = true
				}
			}
			if !matchingJob {
				errorMessages = append(errorMessages, fmt.Sprintf("no jobs match '%s' for group '%s'", jobGlob, group.Name))
			}
		}

		for _, resource := range group.Resources {
			_, exists := c.Resources.Lookup(resource)
			if !exists {
				errorMessages = append(errorMessages, fmt.Sprintf("group '%s' has unknown resource '%s'", group.Name, resource))
			}
		}
	}

	for groupName, groupCount := range groupNames {
		if groupCount > 1 {
			errorMessages = append(errorMessages, fmt.Sprintf("group '%s' appears %d times. Duplicate names are not allowed.", groupName, groupCount))
		}
	}

	if len(c.Groups) != 0 {
		for job, grouped := range jobsGrouped {
			if !grouped {
				errorMessages = append(errorMessages, fmt.Sprintf("job '%s' belongs to no group", job))
			}
		}
	}

	return configErrors, compositeErr(errorMessages)
}

func validateResources(c atc.Config) ([]atc.ConfigErrors, error) {
	var errorMessages []string
	var configErrors []atc.ConfigErrors

	names := map[string]location{}

	for i, resource := range c.Resources {
		location := location{section: "resources", index: i}
		identifier := location.Identifier(resource.Name)

		configError := atc.ValidateIdentifier(resource.Name, identifier)
		if configError != nil {
			configErrors = append(configErrors, *configError)
		}

		if other, exists := names[resource.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"%s and %s have the same name ('%s')",
					other, location, resource.Name))
		} else if resource.Name != "" {
			names[resource.Name] = location
		}

		if resource.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if resource.Type == "" {
			errorMessages = append(errorMessages, identifier+" has no type")
		}
	}

	errorMessages = append(errorMessages, validateResourcesUnused(c)...)

	return configErrors, compositeErr(errorMessages)
}

func validateResourceTypes(c atc.Config, seenTypes map[string]location) ([]atc.ConfigErrors, error) {
	var errorMessages []string
	var configErrors []atc.ConfigErrors

	for i, resourceType := range c.ResourceTypes {
		location := location{section: "resource_types", index: i}
		identifier := location.Identifier(resourceType.Name)

		configError := atc.ValidateIdentifier(resourceType.Name, identifier)
		if configError != nil {
			configErrors = append(configErrors, *configError)
		}

		if other, exists := seenTypes[resourceType.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"%s and %s have the same name ('%s')",
					other, location, resourceType.Name))
		} else if resourceType.Name != "" {
			seenTypes[resourceType.Name] = location
		}

		if resourceType.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if resourceType.Type == "" {
			errorMessages = append(errorMessages, identifier+" has no type")
		}
	}

	return configErrors, compositeErr(errorMessages)
}

func validatePrototypes(c atc.Config, seenTypes map[string]location) ([]atc.ConfigErrors, error) {
	var errorMessages []string
	var configErrors []atc.ConfigErrors

	for i, prototype := range c.Prototypes {
		location := location{section: "prototypes", index: i}
		identifier := location.Identifier(prototype.Name)

		configError := atc.ValidateIdentifier(prototype.Name, identifier)
		if configError != nil {
			configErrors = append(configErrors, *configError)
		}

		if other, exists := seenTypes[prototype.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"%s and %s have the same name ('%s')",
					other, location, prototype.Name))
		} else if prototype.Name != "" {
			seenTypes[prototype.Name] = location
		}

		if prototype.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if prototype.Type == "" {
			errorMessages = append(errorMessages, identifier+" has no type")
		}
	}

	return configErrors, compositeErr(errorMessages)
}

func validateResourcesUnused(c atc.Config) []string {
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

func usedResources(c atc.Config) map[string]bool {
	usedResources := make(map[string]bool)

	for _, job := range c.Jobs {
		_ = job.StepConfig().Visit(atc.StepRecursor{
			OnGet: func(step *atc.GetStep) error {
				usedResources[step.ResourceName()] = true
				return nil
			},
			OnPut: func(step *atc.PutStep) error {
				usedResources[step.ResourceName()] = true
				return nil
			},
		})
	}

	return usedResources
}

func validateJobs(c atc.Config) ([]atc.ConfigErrors, error) {
	var errorMessages []string
	var configErrors []atc.ConfigErrors

	names := map[string]location{}

	if len(c.Jobs) == 0 {
		errorMessages = append(errorMessages, "jobs: pipeline must contain at least one job")
		return configErrors, compositeErr(errorMessages)
	}

	for i, job := range c.Jobs {
		location := location{section: "jobs", index: i}
		identifier := location.Identifier(job.Name)

		configError := atc.ValidateIdentifier(job.Name, identifier)
		if configError != nil {
			configErrors = append(configErrors, *configError)
		}

		if other, exists := names[job.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"%s and %s have the same name ('%s')",
					other, location, job.Name))
		} else if job.Name != "" {
			names[job.Name] = location
		}

		if job.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if job.BuildLogRetention != nil && job.BuildLogsToRetain != 0 {
			errorMessages = append(
				errorMessages,
				fmt.Sprintf("%s can't use both build_log_retention and build_logs_to_retain", identifier),
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

		configErrors = append(configErrors, validator.ConfigErrors...)

		errorMessages = append(errorMessages, validator.Errors...)
	}

	return configErrors, compositeErr(errorMessages)
}

func compositeErr(errorMessages []string) error {
	if len(errorMessages) == 0 {
		return nil
	}

	return errors.New(strings.Join(errorMessages, "\n"))
}

func validateVarSources(c atc.Config) ([]atc.ConfigErrors, error) {
	var errorMessages []string
	var configErrors []atc.ConfigErrors

	names := map[string]location{}

	for i, varSource := range c.VarSources {
		location := location{section: "var_sources", index: i}
		identifier := location.Identifier(varSource.Name)

		configError := atc.ValidateIdentifier(varSource.Name, identifier)
		if configError != nil {
			configErrors = append(configErrors, *configError)
		}

		if factory, exists := creds.ManagerFactories()[varSource.Type]; exists {
			// TODO: this check should eventually be removed once all credential managers
			// are supported in pipeline. - @evanchaoli
			switch varSource.Type {
			case "vault", "dummy", "ssm", "secretsmanager", "idtoken":
			default:
				errorMessages = append(errorMessages, fmt.Sprintf("credential manager type %s is not supported in pipeline yet", varSource.Type))
			}

			if other, ok := names[varSource.Name]; ok {
				errorMessages = append(errorMessages,
					fmt.Sprintf(
						"%s and %s have the same name ('%s')",
						other, location, varSource.Name))
			}
			names[varSource.Name] = location

			if manager, err := factory.NewInstance(varSource.Config); err == nil {
				err = manager.Validate()
				if err != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("credential manager %s is invalid: %s", varSource.Name, err.Error()))
				}
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to create credential manager %s: %s", varSource.Name, err.Error()))
			}
		} else {
			errorMessages = append(errorMessages, fmt.Sprintf("unknown credential manager type: %s", varSource.Type))
		}
	}

	if _, err := c.VarSources.OrderByDependency(); err != nil {
		errorMessages = append(errorMessages, fmt.Sprintf("failed to order by dependency: %s", err.Error()))
	}

	return configErrors, compositeErr(errorMessages)
}

func validateDisplay(c atc.Config) ([]atc.ConfigErrors, error) {
	var configErrors []atc.ConfigErrors

	if c.Display == nil {
		return configErrors, nil
	}

	url, err := url.Parse(c.Display.BackgroundImage)

	if err != nil {
		return configErrors, fmt.Errorf("background_image is not a valid URL: %s", c.Display.BackgroundImage)
	}

	switch url.Scheme {
	case "https":
	case "http":
	case "":
		break
	default:
		return configErrors, fmt.Errorf("background_image scheme must be either http, https or relative")
	}

	return configErrors, nil
}

// JobState represents the visit state of a job during cycle detection
type JobState int

const (
	NotVisited JobState = iota
	Visiting
	Visited
)

// detectCycle performs cycle detection for a single traversal starting from job j
func detectCycle(j atc.JobConfig, visited map[string]JobState, pipelineConfig atc.Config) error {
	if visited[j.Name] == Visiting {
		return fmt.Errorf("pipeline contains a cycle that starts at Job '%s'", j.Name)
	}

	if visited[j.Name] == Visited {
		return nil
	}

	visited[j.Name] = Visiting

	err := j.StepConfig().Visit(atc.StepRecursor{
		OnGet: func(step *atc.GetStep) error {
			for _, nextJobName := range step.Passed {
				nextJob := findJobByName(nextJobName, pipelineConfig.Jobs)
				if err := detectCycle(nextJob, visited, pipelineConfig); err != nil {
					return err
				}
			}
			return nil
		},
	})

	if err != nil {
		return err
	}

	visited[j.Name] = Visited
	return nil
}

// validateCycle checks for cycles in the entire pipeline configuration
func validateCycle(c atc.Config) error {
	for _, job := range c.Jobs {
		// Create fresh visited map for each root job
		visited := make(map[string]JobState)

		if err := detectCycle(job, visited, c); err != nil {
			return err
		}
	}
	return nil
}

func findJobByName(jobName string, jobs atc.JobConfigs) atc.JobConfig {
	for _, currJob := range jobs {
		if jobName == currJob.Name {
			return currJob
		}
	}
	return atc.JobConfig{}
}
