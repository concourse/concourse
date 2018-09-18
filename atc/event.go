package atc

type Event interface {
	EventType() EventType
	Version() EventVersion
}

type EventType string

// semantic version for an individual event.
//
// minor bumps are expected to be backwards-compatible, meaning clients can
// interpret them via the older handlers, and they can unmarshal into the new
// version trivially.
//
// ATC will always emit the highest possible minor version for an event. this is
// so that we don't have to maintain copies of the event every time there's a
// minor bump.
//
// an example of a minor bump would be an additive change, i.e. a new field.
//
// major bumps are backwards-incompatible.
//
// an example of a major bump would be the changing or removal of a field.
type EventVersion string
