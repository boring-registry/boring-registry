package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOidcProviderDiscovery(t *testing.T) {
	data := `{
				"issuer": "ISSUER",
				"authorization_endpoint": "/auth",
				"token_endpoint": "/token",
				"jwks_uri": "/keys",
				"id_token_signing_alg_values_supported": ["RS256"]
			}`

	var issuer string
	hf := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, strings.ReplaceAll(data, "ISSUER", issuer))
	}
	s := httptest.NewServer(http.HandlerFunc(hf))
	defer s.Close()
	issuer = s.URL

	provider, err := NewOidcProvider(context.Background(), issuer, "boring-registry")
	assert.NoError(t, err)
	assert.NotEmpty(t, provider.AuthURL())
	assert.NotEmpty(t, provider.TokenURL())
}
