package radar

//go:generate counterfeiter . Notifications
type Notifications interface {
	Listen(string) (chan bool, error)
	Unlisten(string, chan bool) error
}
