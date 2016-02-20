package rc

type TargetName string

func (name *TargetName) UnmarshalFlag(value string) error {
	*name = TargetName(value)
	return nil
}
