package atc

type User struct {
	ID        int    `json:"id,omitempty"`
	Username  string `json:"username,omitempty"`
	Connector string `json:"connector,omitempty"`
	LastLogin int64  `json:"last_login,omitempty"`
	Sub       string `json:"sub,omitempty"`
}

type UserInfo struct {
	Sub       string              `json:"sub"`
	Name      string              `json:"name"`
	UserId    string              `json:"user_id"`
	UserName  string              `json:"user_name"`
	Email     string              `json:"email"`
	IsAdmin   bool                `json:"is_admin"`
	IsSystem  bool                `json:"is_system"`
	Teams     map[string][]string `json:"teams"`
	Connector string              `json:"connector"`
}

// PreferredUserName returns a user's display name that might not be unique.
// TODO: so far we have only tested with a few connectors, rest connectors
// are handled in default clause. Asking for help to add case clauses for
// other connectors.
func (user UserInfo) PreferredUserName() string {
	switch user.Connector {
	case "local", "ldap":
		return user.Name
	case "github":
		return user.UserName
	default:
		if user.UserName != "" {
			return user.UserName
		}
		if user.Name != "" {
			return user.Name
		}
		return user.UserId
	}
}

// PreferredUserId returns a user's unique id.
// TODO: so far we have only tested with a few connectors, rest connectors
// are handled in default clause. Asking for help to add case clauses for
// other connectors.
func (user UserInfo) PreferredUserId() string {
	switch user.Connector {
	case "local", "ldap":
		return user.UserId
	case "github":
		return user.Name
	default:
		return user.UserId
	}
}
