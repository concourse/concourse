package worker

import "time"

func FinalTTL(ttl time.Duration) *time.Duration {
	return &ttl
}
