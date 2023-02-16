package nodeinfo

const (
	// ExecutorRole configed as execution node
	ExecutorRole = Role(1 << iota)
	// ConsensusRole configed as consensus node
	ConsensusRole
)

type (
	// Role role type of node
	Role uint64
	// Roles a set of Role
	Roles uint64
)

// GenEmptyRoles return an empty Roles
func GenEmptyRoles() Roles {
	return Roles(0)
}

// Has returns true if roles contains the role
func (rs Roles) Has(r Role) bool {
	return uint64(rs)&uint64(r) > 0
}

// Add add role
func (rs *Roles) Add(r Role) {
	*rs = Roles(uint64(*rs) | uint64(r))
}

// Remove remove role
func (rs *Roles) Remove(r Role) {
	*rs = Roles(uint64(*rs) & ^uint64(r))
}

// Clear clear role
func (rs *Roles) Clear() {
	*rs = Roles(0)
}
