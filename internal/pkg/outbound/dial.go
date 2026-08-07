package outbound

import (
	"net/netip"
	"syscall"
)

func ParsePrefixes(values []string) ([]netip.Prefix, error) {
	return parsePrefixes(values)
}

// Control refuses every address this instance has no business reaching. ControlAllowingPrivate
// is the same guard with one exception opened, and it is deliberately a separate call so no
// caller widens the default by passing a flag it did not think about.
func Control(allowed []netip.Prefix) func(string, string, syscall.RawConn) error {
	return control(allowed, false)
}

// ControlAllowingPrivate opens private ranges for one caller that an administrator granted
// the exception. Loopback and link-local stay refused: the first is this instance's own
// internals and the second is where a cloud provider hands out the machine's credentials.
func ControlAllowingPrivate(allowed []netip.Prefix) func(string, string, syscall.RawConn) error {
	return control(allowed, true)
}
