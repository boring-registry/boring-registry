package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-kit/kit/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIssuer serves OIDC discovery and a JWKS, and signs tokens with its key.
type fakeIssuer struct {
	srv *httptest.Server
	key *rsa.PrivateKey
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := &fakeIssuer{key: key}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                f.srv.URL,
				"authorization_endpoint":                f.srv.URL + "/auth",
				"token_endpoint":                        f.srv.URL + "/token",
				"jwks_uri":                              f.srv.URL + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
				{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIssuer) token(t *testing.T, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key},
		(&jose.SignerOptions{}).WithHeader("kid", "k1"))
	require.NoError(t, err)
	base := map[string]any{"iss": f.srv.URL, "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix()}
	for k, v := range claims {
		base[k] = v
	}
	payload, err := json.Marshal(base)
	require.NoError(t, err)
	jws, err := signer.Sign(payload)
	require.NoError(t, err)
	s, err := jws.CompactSerialize()
	require.NoError(t, err)
	return s
}

// captureEndpoint records the context the wrapped endpoint was called with.
func captureEndpoint(got *context.Context) func(context.Context, any) (any, error) {
	return func(ctx context.Context, _ any) (any, error) {
		*got = ctx
		return true, nil
	}
}

func TestMiddlewareAttachesOidcCaller(t *testing.T) {
	f := newFakeIssuer(t)
	p, err := NewOidcProvider(context.Background(), f.srv.URL, "terraform-cli", "terraform-registry")
	require.NoError(t, err)

	cases := []struct {
		name   string
		claims map[string]any
		want   Caller
	}{
		{
			name: "person through terraform login",
			claims: map[string]any{"sub": "u-1", "aud": []string{"terraform-registry", "account"}, "azp": "terraform-cli",
				"preferred_username": "jdoe", "email": "jdoe@example.com"},
			want: Caller{Provider: "oidc", Subject: "u-1", Username: "jdoe", Email: "jdoe@example.com", Client: "terraform-cli"},
		},
		{
			name: "machine reader, client_credentials",
			claims: map[string]any{"sub": "sa-1", "aud": "terraform-registry", "azp": "sac-registry-reader-sac-dev",
				"preferred_username": "service-account-sac-registry-reader-sac-dev"},
			want: Caller{Provider: "oidc", Subject: "sa-1", Username: "service-account-sac-registry-reader-sac-dev", Client: "sac-registry-reader-sac-dev"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got context.Context
			ctx := context.WithValue(context.Background(), jwt.JWTContextKey, f.token(t, tc.claims))
			_, err := Middleware(p)(captureEndpoint(&got))(ctx, nil)
			require.NoError(t, err)
			c, ok := CallerFromContext(got)
			require.True(t, ok)
			assert.Equal(t, tc.want, c)
		})
	}
}

func TestMiddlewareRejectsWithoutCaller(t *testing.T) {
	f := newFakeIssuer(t)
	p, err := NewOidcProvider(context.Background(), f.srv.URL, "terraform-cli", "terraform-registry")
	require.NoError(t, err)

	called := false
	next := func(context.Context, any) (any, error) { called = true; return nil, nil }
	ctx := context.WithValue(context.Background(), jwt.JWTContextKey,
		f.token(t, map[string]any{"sub": "u-1", "aud": "portal-internal"}))
	_, err = Middleware(p)(next)(ctx, nil)
	assert.Error(t, err, "a token without the configured audience is still rejected")
	assert.False(t, called)
}

func TestStaticProviderCallerHasNoIdentity(t *testing.T) {
	var got context.Context
	ctx := context.WithValue(context.Background(), jwt.JWTContextKey, "foo")
	_, err := Middleware(NewStaticProvider("foo"))(captureEndpoint(&got))(ctx, nil)
	require.NoError(t, err)
	c, ok := CallerFromContext(got)
	require.True(t, ok)
	assert.Equal(t, Caller{Provider: "static"}, c, "the static token itself is never recorded")
}

func TestLogAttr(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(io.Writer(&buf), nil))

	log.Info("anonymous", LogAttr(context.Background()))
	assert.NotContains(t, buf.String(), `"caller":`, "no caller group without a caller")

	buf.Reset()
	log.Info("with caller", LogAttr(WithCaller(context.Background(),
		Caller{Provider: "oidc", Subject: "u-1", Email: "jdoe@example.com", Client: "terraform-cli"})))
	line := buf.String()
	assert.True(t, strings.Contains(line, `"caller":{"provider":"oidc","sub":"u-1","email":"jdoe@example.com","client":"terraform-cli"}`), line)
	assert.NotContains(t, line, `"username"`, "empty fields are left out")
}
