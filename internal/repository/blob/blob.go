package blob

import (
	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/objectstore"
	"github.com/usenorn/norn/internal/repository"
)

func New(
	cfg config.Storage,
	attachments config.Attachments,
	grants repository.BlobGrant,
) (repository.Blob, error) {
	if cfg.Backend == config.StorageBackendS3 {
		client, err := objectstore.New(cfg)
		if err != nil {
			return nil, err
		}

		return newObjectStore(client), nil
	}

	return newFilesystem(cfg, attachments, grants)
}
