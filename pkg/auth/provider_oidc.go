package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/boring-registry/boring-registry/pkg/audit"
	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/coreos/go-oidc/v3/oidc"
)

type OidcProvider struct {
	logger             *slog.Logger
	issuer             string
	clientIdentifier   string
	provider           *oidc.Provider
	acceptNonJWTTokens bool
}

type OidcConfig struct {
	ClientID           string
	Issuer             string
	Scopes             []string
	LoginGrants        []string
	LoginPorts         []int
	AcceptNonJWTTokens bool
}

func (o *OidcProvider) GetIssuer() string {
	return o.issuer
}

func (o *OidcProvider) validateNonJWTToken(token string) error {
	if token == "" {
		return fmt.Errorf("empty token")
	}

	if len(token) < 10 {
		return fmt.Errorf("token too short")
	}

	o.logger.Debug("accepting non-JWT token", slog.String("issuer", o.issuer))
	return nil
}

func (o *OidcProvider) Verify(ctx context.Context, token string) error {
	parts := strings.Split(token, ".")
	isJWT := len(parts) == 3

	if !isJWT && o.acceptNonJWTTokens {
		return o.validateNonJWTToken(token)
	}

	if !isJWT {
		return fmt.Errorf("token is not in JWT format and provider does not accept non-JWT tokens")
	}

	oidcConfig := &oidc.Config{
		ClientID: o.clientIdentifier,
	}
	verifier := o.provider.VerifierContext(ctx, oidcConfig)

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return fmt.Errorf("%w: %w", core.ErrInvalidToken, err)
	}

	userCtx, err := o.extractUserContext(idToken)
	if err != nil {
		o.logger.Debug("failed to extract user context", slog.String("err", err.Error()))

	} else {

		o.logger.Debug("extracted user context for audit",
			slog.String("email", userCtx.UserEmail),
			slog.String("subject", userCtx.Subject))
	}

	return nil
}

// extractUserContext extracts user information from OIDC token claims
func (o *OidcProvider) extractUserContext(idToken *oidc.IDToken) (*audit.UserContext, error) {
	var claims struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Subject    string `json:"sub"`
		ClientID   string `json:"aud"`
		Username   string `json:"preferred_username"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	return buildUserContext(
		claims.Email,
		claims.Name,
		claims.GivenName,
		claims.FamilyName,
		claims.Subject,
		o.issuer,
		claims.ClientID,
		claims.Username,
	), nil
}

func (o *OidcProvider) AuthURL() string {
	return o.provider.Endpoint().AuthURL
}

func (o *OidcProvider) TokenURL() string {
	return o.provider.Endpoint().TokenURL
}

func NewOidcProvider(ctx context.Context, issuer, clientIdentifier string, acceptNonJWTTokens bool) (*OidcProvider, error) {
	logger := slog.Default()
	start := time.Now()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}

	logger.Info("finished initializing OIDC provider", slog.String("took", time.Since(start).String()))

	return &OidcProvider{
		logger:             logger,
		issuer:             issuer,
		clientIdentifier:   clientIdentifier,
		provider:           provider,
		acceptNonJWTTokens: acceptNonJWTTokens,
	}, nil
}
