package backend

type InputValidationError struct {
	Message string
}

func (e InputValidationError) Error() string {
	return "input validation error: " + e.Message
}

type ClientError struct {
	InnerError error
}

func (e ClientError) Error() string {
	return "client error: " + e.InnerError.Error()
}
