package outbound

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"syscall"
)

var ErrDestinationRefused = errors.New("that address is not reachable from this instance")

var reservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

func parsePrefixes(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse allowed destination %q: %w", trimmed, err)
		}

		prefixes = append(prefixes, prefix.Masked())
	}

	return prefixes, nil
}

func permitted(address netip.Addr, allowed []netip.Prefix, private bool) bool {
	address = address.Unmap()

	for _, prefix := range allowed {
		if prefix.Contains(address) {
			return true
		}
	}

	// Loopback and link-local stay refused even for a caller granted the private exception.
	// Loopback is this instance's own internals, and link-local is where a cloud provider
	// answers with the credentials of the machine Norn is running on; neither is what
	// "our forge is on the office network" means, and both are what an attacker asks for.
	if !address.IsValid() ||
		address.IsUnspecified() ||
		address.IsLoopback() ||
		address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() ||
		address.IsInterfaceLocalMulticast() ||
		address.IsMulticast() {
		return false
	}

	if address.IsPrivate() {
		return private
	}

	for _, prefix := range reservedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}

	return true
}

func refuse(address netip.Addr) error {
	return fmt.Errorf("%w: %s", ErrDestinationRefused, address)
}

func control(allowed []netip.Prefix, private bool) func(string, string, syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		if network != "tcp4" && network != "tcp6" {
			return fmt.Errorf("%w: %s is not a tcp network", ErrDestinationRefused, network)
		}

		parsed, err := netip.ParseAddrPort(address)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrDestinationRefused, address)
		}

		if !permitted(parsed.Addr(), allowed, private) {
			return refuse(parsed.Addr())
		}

		return nil
	}
}
