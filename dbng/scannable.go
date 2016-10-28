package dbng

type scannable interface {
	Scan(destinations ...interface{}) error
}
