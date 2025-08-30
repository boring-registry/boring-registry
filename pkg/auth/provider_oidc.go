package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

type OidcProvider struct {
	logger           *slog.Logger
	issuer           string
	clientIdentifier string
	provider         *oidc.Provider
}

func (p *OidcProvider) String() string { return "oidc" }

// Unfortunately, it's difficult to write tests for this method, as we would need an OIDC Authorization Server
// to generate valid signed JWTs
func (o *OidcProvider) Verify(ctx context.Context, token string) error {
	oidcConfig := &oidc.Config{
		ClientID: o.clientIdentifier,
	}
	verifier := o.provider.VerifierContext(ctx, oidcConfig)

	// Check method documentation to see what is verified and what not.
	// The returned IdToken can be used to verify claims.
	_, err := verifier.Verify(ctx, token)
	return err
}

func (o *OidcProvider) AuthURL() string {
	return o.provider.Endpoint().AuthURL
}

func (o *OidcProvider) TokenURL() string {
	return o.provider.Endpoint().TokenURL
}

func NewOidcProvider(ctx context.Context, issuer, clientIdentifier string) (*OidcProvider, error) {
	logger := slog.Default()
	start := time.Now()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}

	logger.Info("finished initializing OIDC provider", slog.String("took", time.Since(start).String()))

	return &OidcProvider{
		logger:           logger,
		issuer:           issuer,
		clientIdentifier: clientIdentifier,
		provider:         provider,
	}, nil
}
