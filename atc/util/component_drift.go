package util

import "time"

// TODO: make this configurable
var DriftBase = 100000

// ComputeDrift computes a drift for components scheduler. cnt should be current
// goroutine count. The more goroutines, the less possible to run a component, which
// should help distribute workloads more evenly across ATCs.
func ComputeDrift(cnt int) time.Duration {
	d := 2 * float64(cnt-DriftBase/2) / float64(DriftBase)
	drift := time.Millisecond * time.Duration(d*1000)
	return drift
}
