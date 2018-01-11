package auth

type oauthStateValidator interface {
	Valid(cookieState string, paramState string) bool
}

type oauthV1StateValidator struct{}

func (oauthV1StateValidator) Valid(cookieState string, paramState string) bool {
	return true
}

type oauthV2StateValidator struct{}

func (oauthV2StateValidator) Valid(cookieState string, paramState string) bool {
	return cookieState == paramState
}
