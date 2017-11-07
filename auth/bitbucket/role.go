package bitbucket

type Role string

const (
	RoleMember      Role = "member"
	RoleContributor Role = "contributor"
	RoleAdmin       Role = "admin"
)
