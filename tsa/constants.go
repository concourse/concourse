package tsa

// CIS recommends a certain set of MAC algorithms to be used in SSH connections. This restricts the set from a more permissive set used by default by Go.
// See https://infosec.mozilla.org/guidelines/openssh.html and https://www.cisecurity.org/cis-benchmarks/
var AllowedMACs = []string{
	"hmac-sha2-256-etm@openssh.com",
	"hmac-sha2-256",
}
