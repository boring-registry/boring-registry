package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/boring-registry/boring-registry/pkg/discovery"
)

func findLoginByClient(logins []*discovery.LoginV1, clientID string) bool {
    for _, login := range logins {
        if login.Client == clientID {
            return true
        }
    }
    return false
}

func TestAuthMiddleware(t *testing.T) {
	data := `{
				"issuer": "ISSUER",
				"authorization_endpoint": "/auth",
				"token_endpoint": "/token",
				"jwks_uri": "/keys",
				"id_token_signing_alg_values_supported": ["RS256"]
			}`

	hf := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, strings.ReplaceAll(data, "ISSUER", flagAuthOidcIssuer))
	}
	s := httptest.NewServer(http.HandlerFunc(hf))
	defer s.Close()

	tests := []struct {
		name             string
		authOidcIssuer   string
		authOidcClientId string
		authOktaIssuer   string
		authOktaClientId string
		authOktaAuthz    string
		authOktaToken    string
		wantErr          bool
		errMessage       string
	}{
		{
			name:             "only OIDC is configured",
			authOidcIssuer:   s.URL,
			authOidcClientId: "boring-registry",
		},
		{
			name:             "OIDC and Okta are both configured",
			authOidcIssuer:   s.URL,
			authOidcClientId: "boring-registry",
			authOktaIssuer:   "something",
			wantErr:          true,
			errMessage:       "both OIDC and Okta are configured, only one is allowed at a time",
		},
		{
			name:             "Okta is configured",
			authOktaIssuer:   "something",
			authOktaClientId: "boring-registry",
			authOktaAuthz:    "/authz",
			authOktaToken:    "/token",
		},
	}

	for _, test := range tests {
		// Initializing global variables, this is potentially problematic!
		flagAuthOidcIssuer = test.authOidcIssuer
		flagAuthOidcClientId = test.authOidcClientId
		flagAuthOktaIssuer = test.authOktaIssuer
		flagAuthOktaClientId = test.authOktaClientId
		flagAuthOktaAuthz = test.authOktaAuthz
		flagAuthOktaToken = test.authOktaToken

		mw, logins, err := authMiddleware(context.Background())
		if test.wantErr {
			assert.Error(t, err)
			assert.ErrorContains(t, err, test.errMessage)
		} else {
			assert.NoError(t, err)
			if assert.NotNil(t, logins) {
				if test.authOidcIssuer != "" {
                    assert.True(t, findLoginByClient(logins, test.authOidcClientId), "Expected client not found in logins")
				} else if test.authOktaIssuer != "" {
                    assert.True(t, findLoginByClient(logins, test.authOktaClientId), "Expected client not found in logins")
				}
			    for _, login := range logins {
                    assert.NotEmpty(t, login.Authz)
                    assert.NotEmpty(t, login.Token)
                    assert.NotEmpty(t, login.GrantTypes)
                    assert.NotEmpty(t, login.Ports)
				}
			}
			assert.NotNil(t, mw)
		}
	}
}
