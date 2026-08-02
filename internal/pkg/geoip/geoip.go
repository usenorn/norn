package geoip

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/oschwald/geoip2-golang/v2"

	"github.com/usenorn/norn/internal/config"
)

type Client struct {
	reader *geoip2.Reader
}

func New(cfg config.GeoIP) (*Client, func(), error) {
	if cfg.DatabasePath == "" {
		return &Client{}, func() {}, nil
	}

	reader, err := geoip2.Open(cfg.DatabasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open geoip database %q: %w", cfg.DatabasePath, err)
	}

	cleanup := func() {
		_ = reader.Close()
	}

	return &Client{reader: reader}, cleanup, nil
}

func (c *Client) Enabled() bool {
	return c.reader != nil
}

func (c *Client) Lookup(ip netip.Addr) (countryCode, city string, err error) {
	if c.reader == nil || !ip.IsValid() {
		return "", "", nil
	}

	located, err := c.reader.City(ip)
	if err == nil {
		return located.Country.ISOCode, located.City.Names.English, nil
	}

	var invalidMethod geoip2.InvalidMethodError
	if !errors.As(err, &invalidMethod) {
		return "", "", fmt.Errorf("look up city for %s: %w", ip, err)
	}

	country, err := c.reader.Country(ip)
	if err != nil {
		return "", "", fmt.Errorf("look up country for %s: %w", ip, err)
	}

	return country.Country.ISOCode, "", nil
}
