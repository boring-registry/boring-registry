package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"testing"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/stretchr/testify/assert"
)

type mockProvider struct {
	name          string
	issuer        string
	shouldSucceed bool
	verifyFunc    func(ctx context.Context, token string) error
}

func (m *mockProvider) Verify(ctx context.Context, token string) error {
	if m.verifyFunc != nil {
		return m.verifyFunc(ctx, token)
	}
	if m.shouldSucceed {
		return nil
	}
	return fmt.Errorf("mock verification failed for %s", m.name)
}

func (m *mockProvider) GetIssuer() string {
	return m.issuer
}

type mockOidcProvider struct {
	*mockProvider
}

func (m *mockOidcProvider) GetIssuer() string {
	return m.issuer
}

func createJWTWithIssuer(issuer string) string {
	header := `{"alg":"HS256","typ":"JWT"}`
	payload := fmt.Sprintf(`{"iss":"%s","sub":"1234567890","name":"John Doe","iat":1516239022}`, issuer)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString([]byte("dummy-signature"))

	return fmt.Sprintf("%s.%s.%s", headerB64, payloadB64, signature)
}

func TestParseJWTIssuer(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		expectedIss string
		expectError bool
	}{
		{
			name:        "valid JWT",
			token:       createJWTWithIssuer("https://example.com"),
			expectedIss: "https://example.com",
			expectError: false,
		},
		{
			name:        "malformed JWT only 2 parts",
			token:       "header.payload",
			expectedIss: "",
			expectError: true,
		},
		{
			name:        "malformed JWT 4 parts",
			token:       "header.payload.signature.extra",
			expectedIss: "",
			expectError: true,
		},
		{
			name:        "invalid base64 payload",
			token:       "header.@invalid@base64@.signature",
			expectedIss: "",
			expectError: true,
		},
		{
			name:        "invalid JSON payload",
			token:       fmt.Sprintf("header.%s.signature", base64.RawURLEncoding.EncodeToString([]byte("invalid json"))),
			expectedIss: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuer, err := parseJWTIssuer(tt.token)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIss, issuer)
			}
		})
	}
}

func TestFindMatchingProvider(t *testing.T) {
	oidcProvider1 := &mockOidcProvider{
		mockProvider: &mockProvider{
			name:   "provider1",
			issuer: "https://example1.com",
		},
	}
	oidcProvider2 := &mockOidcProvider{
		mockProvider: &mockProvider{
			name:   "provider2",
			issuer: "https://example2.com",
		},
	}
	staticProvider := &mockProvider{name: "static"}

	providers := []Provider{oidcProvider1, oidcProvider2, staticProvider}

	tests := []struct {
		name     string
		issuer   string
		expected Provider
	}{
		{
			name:     "find matching provider",
			issuer:   "https://example1.com",
			expected: oidcProvider1,
		},
		{
			name:     "find second matching provider",
			issuer:   "https://example2.com",
			expected: oidcProvider2,
		},
		{
			name:     "no matching provider",
			issuer:   "https://unknown.com",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMatchingProvider(providers, tt.issuer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLikelyJWT(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "valid JWT format",
			token:    "header.payload.signature",
			expected: true,
		},
		{
			name:     "invalid JWT format 2 parts",
			token:    "header.payload",
			expected: false,
		},
		{
			name:     "invalid JWT format 4 parts",
			token:    "header.payload.signature.extra",
			expected: false,
		},
		{
			name:     "single token",
			token:    "simpletoken",
			expected: false,
		},
		{
			name:     "empty token",
			token:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLikelyJWT(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	slog.SetDefault(slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError})))

	t.Run("no providers should skip auth", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, "any-token")
		_, err := Middleware()(nopEndpoint)(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("no token in context", func(t *testing.T) {
		provider := &mockProvider{name: "test", shouldSucceed: true}
		ctx := context.Background()
		_, err := Middleware(provider)(nopEndpoint)(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
	})

	t.Run("non-JWT token with supporting provider", func(t *testing.T) {
		nonJWTProvider := &mockProvider{
			name: "nonJWTProvider",
			verifyFunc: func(ctx context.Context, token string) error {
				if token == "ci-system-token" {
					return nil
				}
				return fmt.Errorf("token not supported")
			},
		}
		regularProvider := &mockProvider{name: "regular", shouldSucceed: false}

		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, "ci-system-token")
		_, err := Middleware(nonJWTProvider, regularProvider)(nopEndpoint)(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("JWT with matching issuer", func(t *testing.T) {
		issuer := "https://example.com"
		token := createJWTWithIssuer(issuer)

		matchingProvider := &mockOidcProvider{
			mockProvider: &mockProvider{
				name:          "matching",
				issuer:        issuer,
				shouldSucceed: true,
			},
		}
		nonMatchingProvider := &mockOidcProvider{
			mockProvider: &mockProvider{
				name:          "non-matching",
				issuer:        "https://other.com",
				shouldSucceed: false,
			},
		}

		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, token)
		_, err := Middleware(nonMatchingProvider, matchingProvider)(nopEndpoint)(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("JWT with no matching provider fallback to all providers", func(t *testing.T) {
		issuer := "https://example.com"
		token := createJWTWithIssuer(issuer)

		provider1 := &mockOidcProvider{
			mockProvider: &mockProvider{
				name:          "provider1",
				issuer:        "https://other1.com",
				shouldSucceed: false,
			},
		}
		provider2 := &mockOidcProvider{
			mockProvider: &mockProvider{
				name:          "provider2",
				issuer:        "https://other2.com",
				shouldSucceed: true,
			},
		}

		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, token)
		_, err := Middleware(provider1, provider2)(nopEndpoint)(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("malformed JWT fallback to all providers", func(t *testing.T) {
		token := "malformed.jwt"

		provider1 := &mockProvider{name: "provider1", shouldSucceed: false}
		provider2 := &mockProvider{name: "provider2", shouldSucceed: true}

		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, token)
		_, err := Middleware(provider1, provider2)(nopEndpoint)(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("non-JWT token try all providers", func(t *testing.T) {
		token := "simple-token"

		provider1 := &mockProvider{name: "provider1", shouldSucceed: false}
		provider2 := &mockProvider{name: "provider2", shouldSucceed: true}

		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, token)
		_, err := Middleware(provider1, provider2)(nopEndpoint)(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("all providers fail", func(t *testing.T) {
		token := "invalid-token"

		provider1 := &mockProvider{name: "provider1", shouldSucceed: false}
		provider2 := &mockProvider{name: "provider2", shouldSucceed: false}

		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, token)
		_, err := Middleware(provider1, provider2)(nopEndpoint)(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to verify token")
	})

	t.Run("JWT with matching issuer succeeds even when other providers fail", func(t *testing.T) {
		issuer := "https://example.com"
		token := createJWTWithIssuer(issuer)

		failingProvider := &mockOidcProvider{
			mockProvider: &mockProvider{
				name:          "failing",
				issuer:        "https://other.example.com",
				shouldSucceed: false,
			},
		}
		matchingProvider := &mockOidcProvider{
			mockProvider: &mockProvider{
				name:          "matching",
				issuer:        issuer,
				shouldSucceed: true,
			},
		}

		ctx := context.WithValue(context.Background(), jwt.JWTContextKey, token)
		_, err := Middleware(failingProvider, matchingProvider)(nopEndpoint)(ctx, nil)
		assert.NoError(t, err)
	})
}

func TestAuthMiddlewareWithStaticProvider(t *testing.T) {
	t.Parallel()

	var (
		assert = assert.New(t)
	)

	testCases := []struct {
		name        string
		ctx         context.Context
		token       string
		expectError bool
	}{
		{
			name:        "valid request",
			ctx:         context.WithValue(context.Background(), jwt.JWTContextKey, "foo"),
			token:       "foo",
			expectError: false,
		},
		{
			name:        "invalid request",
			ctx:         context.WithValue(context.Background(), jwt.JWTContextKey, "foo"),
			token:       "bar",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := Middleware(NewStaticProvider(tc.token))(nopEndpoint)(tc.ctx, nil)
			switch tc.expectError {
			case true:
				assert.Error(err)
			case false:
				assert.NoError(err)
			}
		})
	}
}

func nopEndpoint(ctx context.Context, request interface{}) (interface{}, error) {
	return true, nil
}
