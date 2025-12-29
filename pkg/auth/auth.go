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

type TokenClaims struct {
	Issuer string `json:"iss"`
}

func parseJWTIssuer(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed jwt, expected 3 parts got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("malformed jwt payload: %v", err)
	}

	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to unmarshal claims: %v", err)
	}

	return claims.Issuer, nil
}

type IssuerProvider interface {
	Provider
	GetIssuer() string
}

func findMatchingProvider(providers []Provider, issuer string) Provider {
	for _, provider := range providers {
		if issuerProvider, ok := provider.(IssuerProvider); ok {
			if issuerProvider.GetIssuer() == issuer {
				return provider
			}
		}
	}
	return nil
}

func isLikelyJWT(token string) bool {
	parts := strings.Split(token, ".")
	return len(parts) == 3
}

func Middleware(providers ...Provider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			tokenValue := ctx.Value(jwt.JWTContextKey)

			if len(providers) == 0 {
				return next(ctx, request)
			}

			token, exists := tokenValue.(string)
			if !exists {
				return nil, fmt.Errorf("%w: request does not contain a token", core.ErrUnauthorized)
			}

			if isLikelyJWT(token) {
				issuer, err := parseJWTIssuer(token)
				if err != nil {
					slog.Debug("failed to parse JWT issuer", slog.String("err", err.Error()))
				} else {
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
					}
				}

				var lastError error

				// Iterate through all providers and attempt to verify the token
				for _, provider := range providers {
					err := provider.Verify(ctx, token)
					if err != nil {
						slog.Debug("failed to verify token", slog.String("provider", provider.String()), slog.String("err", err.Error()))
						lastError = err
						continue
					} else {
						slog.Debug("successfully verified token", slog.String("provider", provider.String()))
						return next(ctx, request)
					}
				}
				return nil, fmt.Errorf("failed to verify token: %w", lastError)
			} else {
				return nil, fmt.Errorf("%w: request does not contain a token", core.ErrUnauthorized)
			}

			// No provider could verify the token
			return nil, core.ErrUnauthorized
		}
	}
}
