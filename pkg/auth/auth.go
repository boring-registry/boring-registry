package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/audit"
	"github.com/boring-registry/boring-registry/pkg/core"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
)

type TokenClaims struct {
	Issuer     string `json:"iss"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Subject    string `json:"sub"`
	ClientID   string `json:"aud"`
	Username   string `json:"preferred_username"`
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

// parseJWTClaims extracts user information from JWT token for audit logging
func parseJWTClaims(token string) (*audit.UserContext, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed jwt, expected 3 parts got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed jwt payload: %v", err)
	}

	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %v", err)
	}

	userCtx := &audit.UserContext{
		UserEmail: claims.Email,
		UserName:  claims.Name,
		Subject:   claims.Subject,
		Issuer:    claims.Issuer,
		ClientID:  claims.ClientID,
	}

	if userCtx.UserName == "" {
		userCtx.UserName = claims.Username
	}

	if userCtx.UserName == "" && (claims.GivenName != "" || claims.FamilyName != "") {
		userCtx.UserName = strings.TrimSpace(fmt.Sprintf("%s %s", claims.GivenName, claims.FamilyName))
	}

	if userCtx.UserID == "" {
		userCtx.UserID = claims.Email
	}

	return userCtx, nil
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
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			tokenValue := ctx.Value(jwt.JWTContextKey)

			if len(providers) == 0 {
				return next(ctx, request)
			}

			if token, ok := tokenValue.(string); ok {
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

								if userCtx, err := parseJWTClaims(token); err == nil {
									ctx = audit.SetUserInContext(ctx, userCtx)
									slog.Debug("extracted user context for audit",
										slog.String("email", userCtx.UserEmail),
										slog.String("subject", userCtx.Subject))
								}

								return next(ctx, request)
							}
						}
					}
				}

				var lastError error
				for _, provider := range providers {
					err := provider.Verify(ctx, token)
					if err != nil {
						slog.Debug("failed to verify token", slog.String("err", err.Error()))
						lastError = err
						continue
					} else {
						slog.Debug("successfully verified token")

						if isLikelyJWT(token) {
							if userCtx, err := parseJWTClaims(token); err == nil {
								ctx = audit.SetUserInContext(ctx, userCtx)
								slog.Debug("extracted user context for audit",
									slog.String("email", userCtx.UserEmail),
									slog.String("subject", userCtx.Subject))
							}
						}

						return next(ctx, request)
					}
				}
				return nil, fmt.Errorf("failed to verify token: %w", lastError)
			} else {
				return nil, fmt.Errorf("%w: request does not contain a token", core.ErrUnauthorized)
			}

			return nil, core.ErrUnauthorized
		}
	}
}
