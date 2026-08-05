package workspace_test

import (
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func licensedForDirectory() entity.Licence {
	return entity.Licence{
		Holder:    "Northwind Studio",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Features:  entity.LicenceFeatures{Directory: true},
	}
}
