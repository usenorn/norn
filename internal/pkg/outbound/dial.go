package outbound

import (
	"net/netip"
	"syscall"
)

func ParsePrefixes(values []string) ([]netip.Prefix, error) {
	return parsePrefixes(values)
}

func Control(allowed []netip.Prefix) func(string, string, syscall.RawConn) error {
	return control(allowed, false)
}

func ControlAllowingPrivate(allowed []netip.Prefix) func(string, string, syscall.RawConn) error {
	return control(allowed, true)
}
