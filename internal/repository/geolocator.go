package repository

import (
	"context"
	"net/netip"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=geolocator.go -destination=geolocation/mock_geolocator.go -package=geolocation -mock_names=GeoLocator=MockGeoLocator

type GeoLocator interface {
	Locate(ctx context.Context, ip netip.Addr) (entity.Location, error)
}
