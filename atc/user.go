package atc

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type User struct {
	ID        int    `json:"id,omitempty"`
	Username  string `json:"username,omitempty"`
	Connector string `json:"connector,omitempty"`
	LastLogin int64  `json:"last_login,omitempty"`
	Sub       string `json:"sub,omitempty"`
}

type UserInfo struct {
	Sub           string              `json:"sub"`
	Name          string              `json:"name"`
	UserId        string              `json:"user_id"`
	UserName      string              `json:"user_name"`
	Email         string              `json:"email"`
	IsAdmin       bool                `json:"is_admin"`
	IsSystem      bool                `json:"is_system"`
	Teams         map[string][]string `json:"teams"`
	Connector     string              `json:"connector"`
	DisplayUserId string              `json:"display_user_id"`
}

//counterfeiter:generate . DisplayUserIdGenerator
type DisplayUserIdGenerator interface {
	DisplayUserId(connector, userid, username, preferredUsername, email string) string
}
