package atc

type User struct {
	ID        int    `json:"id,omitempty"`
	Username  string `json:"username,omitempty"`
	Connector string `json:"connector,omitempty"`
	LastLogin int64  `json:"last_login,omitempty"`
	Sub       string `json:"sub", omitempty`
}
