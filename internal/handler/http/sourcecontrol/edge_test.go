package sourcecontrol_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/sourcecontrol"
	scmsvc "github.com/usenorn/norn/internal/service/scm"
)

func TestOnlyForgeryIsRefusedAndEveryOtherMissIsAcceptedAndIgnored(t *testing.T) {
	cases := map[string]struct {
		cause  error
		status int
	}{
		"a signature that did not verify": {
			entity.ErrSCMSignatureInvalid, http.StatusUnauthorized,
		},
		"a repository no workspace connected": {
			fmt.Errorf("%w: acme/platform", entity.ErrSCMRepositoryNotFound), http.StatusOK,
		},
		"an installation nobody connected": {
			fmt.Errorf("%w: installation 884411", entity.ErrSCMConnectionNotFound), http.StatusOK,
		},
		"no application registered": {
			entity.ErrSCMAppNotFound, http.StatusOK,
		},
		"a body that names no repository": {
			entity.ErrSCMDeliveryUnroutable, http.StatusOK,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			syncer := scmsvc.NewMockSourceControlSync(ctrl)

			syncer.EXPECT().
				AcceptFromApp(gomock.Any(), entity.SCMProviderGitHub, gomock.Any(), gomock.Any()).
				Return(uuid.Nil, tc.cause)

			edge := sourcecontrol.New(syncer, config.SourceControl{MaxDeliveryBytes: 1 << 20})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(
				http.MethodPost, sourcecontrol.AppDeliveryPath, strings.NewReader("{}"),
			)

			edge.DeliverToApp(recorder, request)

			if recorder.Code != tc.status {
				t.Fatalf(
					"%s answered %d, want %d. An application installed across an account is sent "+
						"every repository's events, so refusing the ones Norn cannot attribute "+
						"paints the whole webhook history red and buries the deliveries that "+
						"really did fail to verify",
					name, recorder.Code, tc.status,
				)
			}
		})
	}
}
