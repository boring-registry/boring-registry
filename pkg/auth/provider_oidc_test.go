package auth

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
		if _, err := io.WriteString(w, strings.ReplaceAll(data, "ISSUER", issuer)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(hf))
	defer s.Close()
	issuer = s.URL

	provider, err := NewOidcProvider(context.Background(), issuer, "boring-registry", false)
	assert.NoError(t, err)
	assert.NotEmpty(t, provider.AuthURL())
	assert.NotEmpty(t, provider.TokenURL())
}

func TestOidcProviderGetIssuer(t *testing.T) {
	issuer := "https://example.com"
	provider := &OidcProvider{
		issuer: issuer,
	}

	assert.Equal(t, issuer, provider.GetIssuer())
}

func TestOidcProviderNonJWTTokenHandling(t *testing.T) {
	tests := []struct {
		name               string
		acceptNonJWTTokens bool
		token              string
		expectError        bool
	}{
		{
			name:               "Non-JWT token accepted when configured",
			acceptNonJWTTokens: true,
			token:              "non-jwt-token-from-ci-system",
			expectError:        false,
		},
		{
			name:               "Non-JWT token rejected when not configured",
			acceptNonJWTTokens: false,
			token:              "non-jwt-token-from-ci-system",
			expectError:        true,
		},
		{
			name:               "Empty token always rejected",
			acceptNonJWTTokens: true,
			token:              "",
			expectError:        true,
		},
		{
			name:               "Short token rejected",
			acceptNonJWTTokens: true,
			token:              "short",
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &OidcProvider{
				logger:             slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError})),
				issuer:             "https://example.com",
				acceptNonJWTTokens: tt.acceptNonJWTTokens,
			}

			err := provider.Verify(context.Background(), tt.token)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOidcProviderValidateNonJWTToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))

	provider := &OidcProvider{
		logger: logger,
		issuer: "https://example.com",
	}

	tests := []struct {
		name        string
		token       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid token",
			token:       "valid-ci-token-1234567890",
			expectError: false,
		},
		{
			name:        "empty token",
			token:       "",
			expectError: true,
			errorMsg:    "empty token",
		},
		{
			name:        "token too short",
			token:       "short",
			expectError: true,
			errorMsg:    "token too short",
		},
		{
			name:        "minimum length token",
			token:       "1234567890",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.validateNonJWTToken(tt.token)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOidcProviderVerify(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("Non-JWT token validation", func(t *testing.T) {
		provider := &OidcProvider{
			logger:             logger,
			issuer:             "https://example.com",
			acceptNonJWTTokens: true,
		}

		err := provider.Verify(context.Background(), "valid-ci-token")
		assert.NoError(t, err)

		err = provider.Verify(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty token")
	})

	t.Run("Regular OIDC token validation", func(t *testing.T) {
		data := `{
			"issuer": "ISSUER",
			"authorization_endpoint": "/auth",
			"token_endpoint": "/token",
			"jwks_uri": "/keys",
			"id_token_signing_alg_values_supported": ["RS256"]
		}`

		jwksData := `{
			"keys": [
				{
					"kty": "RSA",
					"use": "sig",
					"kid": "test-key",
					"n": "example-n",
					"e": "AQAB"
				}
			]
		}`

		var issuer string
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, strings.ReplaceAll(data, "ISSUER", issuer))
		})
		mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, jwksData)
		})

		s := httptest.NewServer(mux)
		defer s.Close()
		issuer = s.URL

		provider, err := NewOidcProvider(context.Background(), issuer, "boring-registry", false)
		assert.NoError(t, err)

		err = provider.Verify(context.Background(), "invalid.jwt.token")
		assert.Error(t, err)
	})
}

func TestNewOidcProvider(t *testing.T) {
	t.Run("successful provider creation", func(t *testing.T) {
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

		provider, err := NewOidcProvider(context.Background(), issuer, "test-client-id", false)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, issuer, provider.GetIssuer())
		assert.Equal(t, "test-client-id", provider.clientIdentifier)
		assert.NotNil(t, provider.logger)
		assert.NotNil(t, provider.provider)
		assert.False(t, provider.acceptNonJWTTokens)
	})

	t.Run("failed provider creation invalid issuer", func(t *testing.T) {
		provider, err := NewOidcProvider(context.Background(), "http://invalid-url-12345", "test-client-id", false)
		assert.Error(t, err)
		assert.Nil(t, provider)
	})

	t.Run("failed provider creation server error", func(t *testing.T) {
		hf := func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		s := httptest.NewServer(http.HandlerFunc(hf))
		defer s.Close()

		provider, err := NewOidcProvider(context.Background(), s.URL, "test-client-id", false)
		assert.Error(t, err)
		assert.Nil(t, provider)
	})

	t.Run("provider with non-JWT token support", func(t *testing.T) {
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

		provider, err := NewOidcProvider(context.Background(), issuer, "test-client-id", true)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
		assert.True(t, provider.acceptNonJWTTokens)
	})
}

func TestOidcProviderURLs(t *testing.T) {
	data := `{
		"issuer": "ISSUER",
		"authorization_endpoint": "ISSUER/auth",
		"token_endpoint": "ISSUER/token", 
		"jwks_uri": "ISSUER/keys",
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

	provider, err := NewOidcProvider(context.Background(), issuer, "test-client", false)
	assert.NoError(t, err)

	authURL := provider.AuthURL()
	tokenURL := provider.TokenURL()

	assert.Equal(t, fmt.Sprintf("%s/auth", issuer), authURL)
	assert.Equal(t, fmt.Sprintf("%s/token", issuer), tokenURL)
}

func TestOidcConfig(t *testing.T) {
	config := OidcConfig{
		ClientID:           "test-client-id",
		Issuer:             "https://example.com",
		Scopes:             []string{"openid", "profile"},
		LoginGrants:        []string{"authorization_code"},
		LoginPorts:         []int{10000, 10010},
		AcceptNonJWTTokens: true,
	}

	assert.Equal(t, "test-client-id", config.ClientID)
	assert.Equal(t, "https://example.com", config.Issuer)
	assert.Equal(t, []string{"openid", "profile"}, config.Scopes)
	assert.Equal(t, []string{"authorization_code"}, config.LoginGrants)
	assert.Equal(t, []int{10000, 10010}, config.LoginPorts)
	assert.True(t, config.AcceptNonJWTTokens)
}
