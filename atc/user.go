package atc

import "time"

type User struct {
	ID        int       `json:"id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Connector string    `json:"connector,omitempty"`
	LastLogin time.Time `json:"last_login,omitempty"`
}
