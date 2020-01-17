package atc

import "time"

type Wall struct {
	Message string        `json:"message,omitempty"`
	TTL     time.Duration `json:"TTL,omitempty"`
}
