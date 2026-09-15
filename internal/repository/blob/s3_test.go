package blob

import (
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/objectstore"
)

func TestAnUploadLinkIsSignedForTheDeclaredLengthSoTheStoreRefusesAnyOther(t *testing.T) {
	client, err := objectstore.New(config.Storage{
		Endpoint:        "https://storage.norn.test",
		Region:          "garage",
		Bucket:          "norn",
		AccessKeyID:     "GKnorn",
		SecretAccessKey: "not-a-secret",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("object store: %v", err)
	}

	ticket, err := newObjectStore(client).PresignPut(t.Context(), "attachments/one/two", 4_096, time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}

	link, err := url.Parse(ticket.URL)
	if err != nil {
		t.Fatalf("the upload link %q does not parse: %v", ticket.URL, err)
	}

	signed := strings.Split(link.Query().Get("X-Amz-SignedHeaders"), ";")
	if !slices.Contains(signed, "content-length") {
		t.Fatalf(
			"the upload link signs %v and not the content length. An unsigned length lets the "+
				"holder of the link put an object of any size, long after the workspace was charged "+
				"only for the size it declared.",
			signed,
		)
	}
}
