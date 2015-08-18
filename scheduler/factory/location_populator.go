package factory

import "github.com/concourse/atc"

//go:generate counterfeiter . LocationPopulator

type LocationPopulator interface {
	PopulateLocations(planSequence *atc.PlanSequence)
}

type locationPopulator struct{}

func NewLocationPopulator() LocationPopulator {
	return &locationPopulator{}
}

func (l locationPopulator) PopulateLocations(planSequence *atc.PlanSequence) {
	p := *planSequence
	stepCount := uint(1)

	for i := 0; i < len(p); i++ {
		plan := p[i]
		location := &atc.Location{
			ID:            stepCount,
			ParentID:      0,
			ParallelGroup: 0,
		}
		stepCount = stepCount + l.populateLocations(&plan, location)
		p[i] = plan
	}
}

func (l locationPopulator) populateLocations(planConfig *atc.PlanConfig, location *atc.Location) uint {
	var stepCount uint
	var parentID uint

	parentID = location.ID
	switch {
	case planConfig.Put != "":
		planConfig.Location = location
		// offset by one for the dependent get that will be added
		stepCount = stepCount + 1

	case planConfig.Do != nil:
		// TODO: Do we actually need to increment these two here? See aggregate location.
		serialGroup := location.ID + 1
		stepCount += 1

		if location.SerialGroup != 0 {
			location.ParentID = location.SerialGroup
		}

		children := *planConfig.Do
		for i := 0; i < len(children); i++ {
			child := children[i]
			childLocation := &atc.Location{
				ID:            location.ID + stepCount + 1,
				ParentID:      location.ParentID,
				ParallelGroup: location.ParallelGroup,
				SerialGroup:   serialGroup,
			}

			if child.Do == nil {
				childLocation.Hook = location.Hook
			}

			stepCount = stepCount + l.populateLocations(&child, childLocation)
			children[i] = child
		}

		parentID = serialGroup

	case planConfig.Try != nil:
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      location.ParentID,
			ParallelGroup: 0,
			Hook:          location.Hook,
		}
		stepCount = stepCount + l.populateLocations(planConfig.Try, childLocation)

	case planConfig.Aggregate != nil:
		// TODO: Do we actually need to increment these two here? See do location.
		parallelGroup := location.ID + 1
		stepCount += 1

		if location.ParallelGroup != 0 {
			location.ParentID = location.ParallelGroup
		}

		children := *planConfig.Aggregate
		for i := 0; i < len(children); i++ {
			child := children[i]
			childLocation := &atc.Location{
				ID:            location.ID + stepCount + 1,
				ParentID:      location.ParentID,
				ParallelGroup: parallelGroup,
				SerialGroup:   location.SerialGroup,
			}

			if child.Aggregate == nil && child.Do == nil {
				childLocation.Hook = location.Hook
			}

			stepCount = stepCount + l.populateLocations(&child, childLocation)
			children[i] = child
		}

		parentID = parallelGroup
	default:
		planConfig.Location = location
	}

	if planConfig.Failure != nil {
		child := planConfig.Failure
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      parentID,
			ParallelGroup: 0,
			Hook:          "failure",
		}
		stepCount = stepCount + l.populateLocations(child, childLocation)
	}
	if planConfig.Success != nil {
		child := planConfig.Success
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      parentID,
			ParallelGroup: 0,
			Hook:          "success",
		}
		stepCount = stepCount + l.populateLocations(child, childLocation)
	}
	if planConfig.Ensure != nil {
		child := planConfig.Ensure
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      parentID,
			ParallelGroup: 0,
			Hook:          "ensure",
		}
		stepCount = stepCount + l.populateLocations(child, childLocation)
	}
	return stepCount + 1
}
