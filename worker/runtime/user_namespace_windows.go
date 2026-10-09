//go:build windows

package runtime

var _ UserNamespace = (*userNamespace)(nil)

type userNamespace struct{}

func NewUserNamespace() UserNamespace {
	return &userNamespace{}
}

// MaxValidIds is only meaningful for Linux user namespaces; Windows
// containers don't use uid/gid mappings.
func (s *userNamespace) MaxValidIds() (uint32, uint32, error) {
	return 0, 0, nil
}
