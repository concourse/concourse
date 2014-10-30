package db

type NamedLock interface {
	Name() string
}

type ResourceLock string

func (resourceLock ResourceLock) Name() string {
	return "resource: " + string(resourceLock)
}
