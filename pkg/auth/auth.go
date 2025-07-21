package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/core"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
)

// TokenClaims represents the basic JWT claims we need to extract the issuer
type TokenClaims struct {
	Issuer string `json:"iss"`
}

// parseJWTIssuer extracts the issuer claim from a JWT token without full verification
func parseJWTIssuer(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed jwt, expected 3 parts got %d", len(parts))
	}

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("malformed jwt payload: %v", err)
	}

	// Parse the claims
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to unmarshal claims: %v", err)
	}

	return claims.Issuer, nil
}

// findMatchingProvider finds the OIDC provider that matches the token's issuer
func findMatchingProvider(providers []Provider, issuer string) Provider {
	for _, provider := range providers {
		if oidcProvider, ok := provider.(*OidcProvider); ok {
			if oidcProvider.Issuer == issuer {
				return provider
			}
		}
	}
	return nil
}

// findSemaphoreProvider finds the Semaphore OIDC provider
func findSemaphoreProvider(providers []Provider) Provider {
	for _, provider := range providers {
		if oidcProvider, ok := provider.(*OidcProvider); ok {
			if strings.Contains(oidcProvider.Issuer, "semaphore.ci.confluent.io") {
				return provider
			}
		}
	}
	return nil
}

// isLikelyJWT checks if a token looks like a JWT (has 3 parts separated by dots)
func isLikelyJWT(token string) bool {
	parts := strings.Split(token, ".")
	return len(parts) == 3
}

func Middleware(providers ...Provider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			tokenValue := ctx.Value(jwt.JWTContextKey)

			// Skip any authorization checks, as there are no providers defined
			if len(providers) == 0 {
				return next(ctx, request)
			}

			if token, ok := tokenValue.(string); ok {
				// First, check if we have a Semaphore provider and try it first
				semaphoreProvider := findSemaphoreProvider(providers)
				if semaphoreProvider != nil {
					slog.Debug("trying Semaphore provider first")
					err := semaphoreProvider.Verify(ctx, token)
					if err != nil {
						slog.Debug("failed to verify token with Semaphore provider", slog.String("err", err.Error()))
					} else {
						slog.Debug("successfully verified token with Semaphore provider")
						return next(ctx, request)
					}
				}

				// If Semaphore verification failed or no Semaphore provider, check if token looks like a JWT
				if isLikelyJWT(token) {
					// Try to extract issuer from the JWT token
					issuer, err := parseJWTIssuer(token)
					if err != nil {
						slog.Debug("failed to parse JWT issuer", slog.String("err", err.Error()))
						// Fall back to trying all providers if we can't parse the issuer
						var lastError error
						for _, provider := range providers {
							// Skip Semaphore provider as we already tried it
							if semaphoreProvider != nil && provider == semaphoreProvider {
								continue
							}
							err := provider.Verify(ctx, token)
							if err != nil {
								slog.Debug("failed to verify token", slog.String("err", err.Error()))
								lastError = err
								continue
							} else {
								slog.Debug("successfully verified token")
								return next(ctx, request)
							}
						}
						return nil, fmt.Errorf("failed to verify token: %w", lastError)
					}

					// Find the matching provider based on issuer
					matchingProvider := findMatchingProvider(providers, issuer)
					if matchingProvider != nil {
						slog.Debug("found matching provider for issuer", slog.String("issuer", issuer))
						err := matchingProvider.Verify(ctx, token)
						if err != nil {
							slog.Debug("failed to verify token with matching provider", slog.String("issuer", issuer), slog.String("err", err.Error()))
							return nil, fmt.Errorf("failed to verify token: %w", err)
						} else {
							slog.Debug("successfully verified token with matching provider", slog.String("issuer", issuer))
							return next(ctx, request)
						}
					} else {
						slog.Debug("no matching provider found for issuer", slog.String("issuer", issuer))
						// Fall back to trying all providers if no matching provider found
						var lastError error
						for _, provider := range providers {
							// Skip Semaphore provider as we already tried it
							if semaphoreProvider != nil && provider == semaphoreProvider {
								continue
							}
							err := provider.Verify(ctx, token)
							if err != nil {
								slog.Debug("failed to verify token", slog.String("err", err.Error()))
								lastError = err
								continue
							} else {
								slog.Debug("successfully verified token")
								return next(ctx, request)
							}
						}
						return nil, fmt.Errorf("failed to verify token: %w", lastError)
					}
				} else {
					// Token doesn't look like a JWT - try to verify it with each provider
					slog.Debug("token doesn't appear to be JWT format, trying all providers")
					var lastError error
					
					// Try all providers (except Semaphore which we already tried)
					for _, provider := range providers {
						// Skip Semaphore provider as we already tried it
						if semaphoreProvider != nil && provider == semaphoreProvider {
							continue
						}
						
						err := provider.Verify(ctx, token)
						if err != nil {
							slog.Debug("failed to verify token", slog.String("err", err.Error()))
							lastError = err
							continue
						} else {
							slog.Debug("successfully verified token")
							return next(ctx, request)
						}
					}
					return nil, fmt.Errorf("failed to verify token: %w", lastError)
				}
			} else {
				return nil, fmt.Errorf("%w: request does not contain a token", core.ErrUnauthorized)
			}

			return nil, core.ErrUnauthorized
		}
	}
}
