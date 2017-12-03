package cloud

type Role string

const (
	RoleMember      Role = "member"
	RoleContributor Role = "contributor"
	RoleAdmin       Role = "admin"
)

func (r Role) String() string {
	return string(r)
}
