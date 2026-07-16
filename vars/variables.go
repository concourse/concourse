package vars

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Variables
type Variables interface {
	Get(Reference) (any, bool, error)
	List() ([]Reference, error)
}
