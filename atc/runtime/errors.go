package runtime

type ExecutableNotFoundError struct {
	Message string
}

func (err ExecutableNotFoundError) Error() string {
	return err.Message
}
