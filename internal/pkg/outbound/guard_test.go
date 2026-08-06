package outbound

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
)

func settings() config.Webhooks {
	return config.Webhooks{
		RequestTimeout:  2 * time.Second,
		DialTimeout:     time.Second,
		MaxResponseSize: 4 << 10,
	}
}

func TestTheGuardRefusesEveryAddressThatCouldReachInside(t *testing.T) {
	for _, address := range []string{
		"0.0.0.0",
		"127.0.0.1",
		"127.1.2.3",
		"::1",
		"10.1.2.3",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"fe80::1",
		"fc00::1",
		"fd12:3456::1",
		"100.64.1.1",
		"224.0.0.1",
		"ff02::1",
		"::ffff:127.0.0.1",
		"::ffff:10.0.0.1",
		"2001:db8::1",
		"64:ff9b::7f00:1",
		"192.0.2.5",
		"240.0.0.1",
	} {
		parsed, err := netip.ParseAddr(address)
		if err != nil {
			t.Fatalf("parse %s: %v", address, err)
		}

		if permitted(parsed, nil) {
			t.Errorf(
				"%s is reachable. A destination on it lets anyone who can register a webhook use "+
					"this instance to probe the network it sits in, which is the whole reason the "+
					"guard exists.",
				address,
			)
		}
	}
}

func TestTheGuardPermitsOrdinaryPublicAddresses(t *testing.T) {
	for _, address := range []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34",
		"2606:4700:4700::1111",
		"2a00:1450:4001:800::200e",
	} {
		parsed, err := netip.ParseAddr(address)
		if err != nil {
			t.Fatalf("parse %s: %v", address, err)
		}

		if !permitted(parsed, nil) {
			t.Errorf("%s was refused, so no ordinary receiver could be reached", address)
		}
	}
}

func TestAnOperatorAllowlistReopensExactlyWhatItNames(t *testing.T) {
	allowed, err := parsePrefixes([]string{"10.42.0.0/16", " 192.168.7.5/32 "})
	if err != nil {
		t.Fatalf("parse prefixes: %v", err)
	}

	for _, probe := range []struct {
		address string
		reached bool
	}{
		{"10.42.0.7", true},
		{"10.42.255.254", true},
		{"192.168.7.5", true},
		{"10.43.0.7", false},
		{"192.168.7.6", false},
		{"127.0.0.1", false},
	} {
		parsed, err := netip.ParseAddr(probe.address)
		if err != nil {
			t.Fatalf("parse %s: %v", probe.address, err)
		}

		if permitted(parsed, allowed) != probe.reached {
			t.Errorf(
				"with 10.42.0.0/16 and 192.168.7.5/32 allowed, %s reachable = %v, want %v",
				probe.address, permitted(parsed, allowed), probe.reached,
			)
		}
	}
}

func TestAnUnparsableAllowlistEntryIsRefusedRatherThanIgnored(t *testing.T) {
	if _, err := parsePrefixes([]string{"10.42.0.0"}); err == nil {
		t.Fatal(
			"an address without a prefix length was accepted. Silently dropping it would leave an " +
				"operator believing they had opened a hole they had not, and debugging that means " +
				"reading this code.",
		)
	}

	if _, err := New(config.Webhooks{
		RequestTimeout:      time.Second,
		DialTimeout:         time.Second,
		MaxResponseSize:     1024,
		AllowedDestinations: []string{"not-a-prefix"},
	}); err == nil {
		t.Fatal("a client was built over an unparsable allowlist")
	}
}

func TestALoopbackReceiverIsRefusedAtDialTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(settings())
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	response, err := client.Do(request)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal(
			"a request to a loopback server succeeded. The dial guard is what makes a destination " +
				"pointing at an internal address unusable, and a URL check alone cannot do it " +
				"because a public hostname can resolve to a private address at delivery time.",
		)
	}

	if !errors.Is(err, ErrDestinationRefused) {
		t.Errorf("loopback was refused with %v, which does not report itself as a refused destination", err)
	}
}

func TestAnAllowlistedLoopbackReceiverIsReached(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	permissive := settings()
	permissive.AllowedDestinations = []string{"127.0.0.0/8", "::1/128"}

	client, err := New(permissive)
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf(
			"an explicitly allowlisted loopback destination was still refused: %v. A self-hoster "+
				"whose receiver runs beside Norn has no other way to be reached.",
			err,
		)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusAccepted {
		t.Errorf("allowlisted destination answered %d, want %d", response.StatusCode, http.StatusAccepted)
	}
}

func TestCheckRefusesBeforeAnythingIsDialled(t *testing.T) {
	client, err := New(settings())
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	for _, destination := range []string{
		"http://127.0.0.1:9000/hooks",
		"https://10.0.0.5/hooks",
		"https://[::1]/hooks",
		"https://169.254.169.254/latest/meta-data",
	} {
		if err := client.Check(t.Context(), destination); !errors.Is(err, ErrDestinationRefused) {
			t.Errorf(
				"registering %q reported %v. Refusing at registration is what gives an "+
					"administrator a reason at the moment they paste the URL.",
				destination, err,
			)
		}
	}

	if err := client.Check(t.Context(), "https://"); err == nil {
		t.Error("a URL naming no host was accepted")
	}
}

func TestARedirectIsReportedRatherThanFollowed(t *testing.T) {
	var reached bool

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	permissive := settings()
	permissive.AllowedDestinations = []string{"127.0.0.0/8", "::1/128"}

	client, err := New(permissive)
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, redirector.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("post to a redirecting receiver: %v", err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusFound {
		t.Errorf("a redirect surfaced as %d rather than being reported as-is", response.StatusCode)
	}

	if reached {
		t.Fatal(
			"the redirect was followed. Each hop would need the same address check, and a receiver " +
				"that redirects is a receiver whose operator should be told rather than indulged.",
		)
	}
}

func TestAResponseBodyIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("a", 1<<20)))
	}))
	defer server.Close()

	permissive := settings()
	permissive.AllowedDestinations = []string{"127.0.0.0/8", "::1/128"}
	permissive.MaxResponseSize = 1024

	client, err := New(permissive)
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("post to a talkative receiver: %v", err)
	}

	defer func() { _ = response.Body.Close() }()

	body := make([]byte, 4096)
	read, _ := response.Body.Read(body)

	if read > 1024 {
		t.Errorf("read %d bytes from a receiver capped at 1024; a hostile endpoint could exhaust memory", read)
	}
}

func TestTheClientTakesNoProxyFromTheEnvironment(t *testing.T) {
	client, err := New(settings())
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	capped, ok := client.http.Transport.(*cappedTransport)
	if !ok {
		t.Fatal("the transport is not the capped one, so response bodies are unbounded")
	}

	transport, ok := capped.inner.(*http.Transport)
	if !ok {
		t.Fatal("the inner transport is not an *http.Transport")
	}

	if transport.Proxy != nil {
		t.Fatal(
			"the transport honours a proxy. HTTP_PROXY in the environment would route every " +
				"delivery through a host the dial guard never inspects, defeating all of it.",
		)
	}

	if transport.DialContext == nil {
		t.Fatal("the transport has no DialContext, so the address guard never runs")
	}
}

func TestControlRefusesNetworksThatAreNotTCP(t *testing.T) {
	guard := control(nil)

	if err := guard("udp", "8.8.8.8:53", nil); !errors.Is(err, ErrDestinationRefused) {
		t.Errorf("a udp dial was permitted with %v", err)
	}

	if err := guard("tcp4", "not-an-address", nil); !errors.Is(err, ErrDestinationRefused) {
		t.Errorf("an unparsable address was permitted with %v", err)
	}

	if err := guard("tcp4", "8.8.8.8:443", nil); err != nil {
		t.Errorf("an ordinary public dial was refused: %v", err)
	}
}
