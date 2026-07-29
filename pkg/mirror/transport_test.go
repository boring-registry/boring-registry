package mirror

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/auth"
	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const stubArchiveLocation = "https://example.com/terraform-provider-null_3.3.0_linux_amd64.zip"

// archiveOnlyService implements Service, but only serves the provider archive endpoint.
type archiveOnlyService struct{}

func (s *archiveOnlyService) ListProviderVersions(_ context.Context, _ *core.Provider) (*ListProviderVersionsResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *archiveOnlyService) ListProviderInstallation(_ context.Context, _ *core.Provider) (*ListProviderInstallationResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *archiveOnlyService) RetrieveProviderArchive(_ context.Context, _ *core.Provider) (*retrieveProviderArchiveResponse, error) {
	return &retrieveProviderArchiveResponse{
		location:     stubArchiveLocation,
		mirrorSource: mirrorSource{isMirror: true},
	}, nil
}

// TestRetrieveProviderArchiveAuthentication asserts that the provider archive endpoint accepts
// the token from either the Authorization header or the `token` query parameter.
func TestRetrieveProviderArchiveAuthentication(t *testing.T) {
	t.Parallel()

	const (
		token = "very-secure-token"
		path  = "/registry.terraform.io/hashicorp/null/terraform-provider-null_3.3.0_linux_amd64.zip"
	)

	metrics := o11y.NewMetrics([]float64{1})
	handler := MakeHandler(
		&archiveOnlyService{},
		auth.Middleware(auth.NewStaticProvider(token)),
		metrics.Mirror,
		o11y.NewMiddleware(metrics.Http),
		httptransport.ServerErrorEncoder(ErrorEncoder),
	)

	testCases := []struct {
		name           string
		authorization  string
		query          string
		wantStatusCode int
	}{
		{
			name:           "token in the Authorization header",
			authorization:  "Bearer " + token,
			wantStatusCode: http.StatusTemporaryRedirect,
		},
		{
			name:           "token in the query parameter",
			query:          "?token=" + token,
			wantStatusCode: http.StatusTemporaryRedirect,
		},
		{
			name:           "token in both the Authorization header and the query parameter",
			authorization:  "Bearer " + token,
			query:          "?token=" + token,
			wantStatusCode: http.StatusTemporaryRedirect,
		},
		{
			name:           "no token",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid token in the Authorization header",
			authorization:  "Bearer invalid",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid token in the query parameter",
			query:          "?token=invalid",
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, path+tc.query, nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			require.Equal(t, tc.wantStatusCode, recorder.Code)
			if tc.wantStatusCode == http.StatusTemporaryRedirect {
				assert.Equal(t, stubArchiveLocation, recorder.Header().Get("Location"))
			}
		})
	}
}
