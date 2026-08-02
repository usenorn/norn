package geolocation

import (
	"context"
	"net/netip"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/geoip"
	"github.com/usenorn/norn/internal/repository"
)

type geoLocatorRepository struct {
	client *geoip.Client
}

func New(client *geoip.Client) repository.GeoLocator {
	return &geoLocatorRepository{client: client}
}

func (r *geoLocatorRepository) Locate(_ context.Context, ip netip.Addr) (entity.Location, error) {
	countryCode, city, err := r.client.Lookup(ip)
	if err != nil {
		return entity.Location{}, err
	}

	return entity.Location{CountryCode: countryCode, City: city}, nil
}
