package atc

import "encoding/json"

func (plan Plan) Public() *json.RawMessage {
	var public struct {
		ID PlanID `json:"id"`

		Aggregate      *json.RawMessage `json:"aggregate,omitempty"`
		InParallel     *json.RawMessage `json:"in_parallel,omitempty"`
		Across         *json.RawMessage `json:"across,omitempty"`
		Do             *json.RawMessage `json:"do,omitempty"`
		Get            *json.RawMessage `json:"get,omitempty"`
		Put            *json.RawMessage `json:"put,omitempty"`
		Check          *json.RawMessage `json:"check,omitempty"`
		Task           *json.RawMessage `json:"task,omitempty"`
		SetPipeline    *json.RawMessage `json:"set_pipeline,omitempty"`
		LoadVar        *json.RawMessage `json:"load_var,omitempty"`
		OnAbort        *json.RawMessage `json:"on_abort,omitempty"`
		OnError        *json.RawMessage `json:"on_error,omitempty"`
		Ensure         *json.RawMessage `json:"ensure,omitempty"`
		OnSuccess      *json.RawMessage `json:"on_success,omitempty"`
		OnFailure      *json.RawMessage `json:"on_failure,omitempty"`
		Try            *json.RawMessage `json:"try,omitempty"`
		DependentGet   *json.RawMessage `json:"dependent_get,omitempty"`
		Timeout        *json.RawMessage `json:"timeout,omitempty"`
		Retry          *json.RawMessage `json:"retry,omitempty"`
		ArtifactInput  *json.RawMessage `json:"artifact_input,omitempty"`
		ArtifactOutput *json.RawMessage `json:"artifact_output,omitempty"`
	}

	public.ID = plan.ID

	if plan.Aggregate != nil {
		public.Aggregate = plan.Aggregate.Public()
	}

	if plan.InParallel != nil {
		public.InParallel = plan.InParallel.Public()
	}

	if plan.Across != nil {
		public.Across = plan.Across.Public()
	}

	if plan.Do != nil {
		public.Do = plan.Do.Public()
	}

	if plan.Get != nil {
		public.Get = plan.Get.Public()
	}

	if plan.Put != nil {
		public.Put = plan.Put.Public()
	}

	if plan.Check != nil {
		public.Check = plan.Check.Public()
	}

	if plan.Task != nil {
		public.Task = plan.Task.Public()
	}

	if plan.SetPipeline != nil {
		public.SetPipeline = plan.SetPipeline.Public()
	}

	if plan.LoadVar != nil {
		public.LoadVar = plan.LoadVar.Public()
	}

	if plan.OnAbort != nil {
		public.OnAbort = plan.OnAbort.Public()
	}

	if plan.OnError != nil {
		public.OnError = plan.OnError.Public()
	}

	if plan.Ensure != nil {
		public.Ensure = plan.Ensure.Public()
	}

	if plan.OnSuccess != nil {
		public.OnSuccess = plan.OnSuccess.Public()
	}

	if plan.OnFailure != nil {
		public.OnFailure = plan.OnFailure.Public()
	}

	if plan.Try != nil {
		public.Try = plan.Try.Public()
	}

	if plan.Timeout != nil {
		public.Timeout = plan.Timeout.Public()
	}

	if plan.Retry != nil {
		public.Retry = plan.Retry.Public()
	}

	if plan.ArtifactInput != nil {
		public.ArtifactInput = plan.ArtifactInput.Public()
	}

	if plan.ArtifactOutput != nil {
		public.ArtifactOutput = plan.ArtifactOutput.Public()
	}

	if plan.DependentGet != nil {
		public.DependentGet = plan.DependentGet.Public()
	}

	return enc(public)
}

func (plan AggregatePlan) Public() *json.RawMessage {
	public := make([]*json.RawMessage, len(plan))

	for i := 0; i < len(plan); i++ {
		public[i] = plan[i].Public()
	}

	return enc(public)
}

func (plan InParallelPlan) Public() *json.RawMessage {
	steps := make([]*json.RawMessage, len(plan.Steps))

	for i := 0; i < len(plan.Steps); i++ {
		steps[i] = plan.Steps[i].Public()
	}

	return enc(struct {
		Steps    []*json.RawMessage `json:"steps"`
		Limit    int                `json:"limit,omitempty"`
		FailFast bool               `json:"fail_fast,omitempty"`
	}{
		Steps:    steps,
		Limit:    plan.Limit,
		FailFast: plan.FailFast,
	})
}

func (plan AcrossPlan) Public() *json.RawMessage {
	type scopedStep struct {
		Step   *json.RawMessage `json:"step"`
		Values []interface{}    `json:"values"`
	}

	steps := []scopedStep{}
	for _, step := range plan.Steps {
		steps = append(steps, scopedStep{
			Step:   step.Step.Public(),
			Values: step.Values,
		})
	}

	return enc(struct {
		Vars        []AcrossVar  `json:"vars"`
		Steps       []scopedStep `json:"steps"`
		FailFast    bool         `json:"fail_fast,omitempty"`
	}{
		Vars:        plan.Vars,
		Steps:       steps,
		FailFast:    plan.FailFast,
	})
}

func (plan DoPlan) Public() *json.RawMessage {
	public := make([]*json.RawMessage, len(plan))

	for i := 0; i < len(plan); i++ {
		public[i] = plan[i].Public()
	}

	return enc(public)
}

func (plan EnsurePlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"ensure"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan GetPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string   `json:"type"`
		Name     string   `json:"name,omitempty"`
		Resource string   `json:"resource"`
		Version  *Version `json:"version,omitempty"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
		Version:  plan.Version,
	})
}

func (plan DependentGetPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string `json:"type"`
		Name     string `json:"name,omitempty"`
		Resource string `json:"resource"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
	})
}

func (plan OnAbortPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_abort"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan OnErrorPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_error"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan OnFailurePlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_failure"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan OnSuccessPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_success"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan PutPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string `json:"type"`
		Name     string `json:"name,omitempty"`
		Resource string `json:"resource"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
	})
}

func (plan CheckPlan) Public() *json.RawMessage {
	return enc(struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
	}{
		Type: plan.Type,
		Name: plan.Name,
	})
}

func (plan TaskPlan) Public() *json.RawMessage {
	return enc(struct {
		Name       string `json:"name"`
		Privileged bool   `json:"privileged"`
	}{
		Name:       plan.Name,
		Privileged: plan.Privileged,
	})
}

func (plan SetPipelinePlan) Public() *json.RawMessage {
	return enc(struct {
		Name string `json:"name"`
		Team string `json:"team"`
	}{
		Name: plan.Name,
		Team: plan.Team,
	})
}

func (plan LoadVarPlan) Public() *json.RawMessage {
	return enc(struct {
		Name string `json:"name"`
	}{
		Name: plan.Name,
	})
}

func (plan TimeoutPlan) Public() *json.RawMessage {
	return enc(struct {
		Step     *json.RawMessage `json:"step"`
		Duration string           `json:"duration"`
	}{
		Step:     plan.Step.Public(),
		Duration: plan.Duration,
	})
}

func (plan TryPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
	}{
		Step: plan.Step.Public(),
	})
}

func (plan RetryPlan) Public() *json.RawMessage {
	public := make([]*json.RawMessage, len(plan))

	for i := 0; i < len(plan); i++ {
		public[i] = plan[i].Public()
	}

	return enc(public)
}

func (plan ArtifactInputPlan) Public() *json.RawMessage {
	return enc(plan)
}

func (plan ArtifactOutputPlan) Public() *json.RawMessage {
	return enc(plan)
}

func enc(public interface{}) *json.RawMessage {
	enc, _ := json.Marshal(public)
	return (*json.RawMessage)(&enc)
}
