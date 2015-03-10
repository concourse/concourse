package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/atc"
)

type InvalidConfigError struct {
	GroupsErr    error
	ResourcesErr error
	JobsErr      error
}

func (err InvalidConfigError) Error() string {
	errorMsgs := []string{"invalid configration:"}

	if err.GroupsErr != nil {
		errorMsgs = append(errorMsgs, indent(fmt.Sprintf("invalid groups:\n%s\n", indent(err.GroupsErr.Error()))))
	}

	if err.ResourcesErr != nil {
		errorMsgs = append(errorMsgs, indent(fmt.Sprintf("invalid resources:\n%s\n", indent(err.ResourcesErr.Error()))))
	}

	if err.JobsErr != nil {
		errorMsgs = append(errorMsgs, indent(fmt.Sprintf("invalid jobs:\n%s\n", indent(err.JobsErr.Error()))))
	}

	return strings.Join(errorMsgs, "\n")
}

func indent(msgs string) string {
	lines := strings.Split(msgs, "\n")
	indented := make([]string, len(lines))

	for i, l := range lines {
		indented[i] = "\t" + l
	}

	return strings.Join(indented, "\n")
}

func ValidateConfig(c atc.Config) error {
	groupsErr := validateGroups(c)
	resourcesErr := validateResources(c)
	jobsErr := validateJobs(c)

	if groupsErr == nil && resourcesErr == nil && jobsErr == nil {
		return nil
	}

	return InvalidConfigError{
		GroupsErr:    groupsErr,
		ResourcesErr: resourcesErr,
		JobsErr:      jobsErr,
	}
}

func validateGroups(c atc.Config) error {
	errorMessages := []string{}

	for _, group := range c.Groups {
		for _, job := range group.Jobs {
			_, exists := c.Jobs.Lookup(job)
			if !exists {
				errorMessages = append(errorMessages,
					fmt.Sprintf("group '%s' has unknown job '%s'", group.Name, job))
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

	return compositeErr(errorMessages)
}

func validateResources(c atc.Config) error {
	errorMessages := []string{}

	names := map[string]int{}

	for i, resource := range c.Resources {
		var identifier string
		if resource.Name == "" {
			identifier = fmt.Sprintf("resource at index %d", i)
		} else {
			identifier = fmt.Sprintf("resource '%s'", resource.Name)
		}

		if other, exists := names[resource.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"resources at index %d and %d have the same name ('%s')",
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

func validateJobs(c atc.Config) error {
	errorMessages := []string{}

	names := map[string]int{}

	for i, job := range c.Jobs {
		var identifier string
		if job.Name == "" {
			identifier = fmt.Sprintf("job at index %d", i)
		} else {
			identifier = fmt.Sprintf("job '%s'", job.Name)
		}

		if other, exists := names[job.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"jobs at index %d and %d have the same name ('%s')",
					other, i, job.Name))
		} else if job.Name != "" {
			names[job.Name] = i
		}

		if job.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		for i, input := range job.InputConfigs {
			var inputIdentifier string
			if input.Name() == "" {
				inputIdentifier = fmt.Sprintf("at index %d", i)
			} else {
				inputIdentifier = fmt.Sprintf("'%s'", input.Name())
			}

			if input.Resource == "" {
				errorMessages = append(errorMessages,
					identifier+" has an input ("+inputIdentifier+") with no resource")
			} else {
				_, found := c.Resources.Lookup(input.Resource)
				if !found {
					errorMessages = append(errorMessages,
						fmt.Sprintf(
							"%s has an input (%s) with an unknown resource ('%s')",
							identifier, inputIdentifier, input.Resource))
				}
			}

			for _, job := range input.Passed {
				_, found := c.Jobs.Lookup(job)
				if !found {
					errorMessages = append(errorMessages,
						fmt.Sprintf(
							"%s has an input (%s) with an unknown job dependency ('%s')",
							identifier, inputIdentifier, job))
				}
			}
		}

		for i, output := range job.OutputConfigs {
			outputIdentifier := fmt.Sprintf("at index %d", i)

			if output.Resource == "" {
				errorMessages = append(errorMessages,
					identifier+" has an output ("+outputIdentifier+") with no resource")
			} else {
				_, found := c.Resources.Lookup(output.Resource)
				if !found {
					errorMessages = append(errorMessages,
						fmt.Sprintf(
							"%s has an output (%s) with an unknown resource ('%s')",
							identifier, outputIdentifier, output.Resource))
				}
			}
		}
	}

	return compositeErr(errorMessages)
}

func compositeErr(errorMessages []string) error {
	if len(errorMessages) == 0 {
		return nil
	}

	return errors.New(strings.Join(errorMessages, "\n"))
}
