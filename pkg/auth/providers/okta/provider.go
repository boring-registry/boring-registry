package okta

import (
	"context"

	"github.com/TierMobility/boring-registry/pkg/auth"
	jwtverifier "github.com/okta/okta-jwt-verifier-golang"
)

type Provider struct {
	issuer string
	claims map[string]string
}

func (p *Provider) Verify(ctx context.Context, token string) error {
	opts := jwtverifier.JwtVerifier{
		Issuer:           p.issuer,
		ClaimsToValidate: p.claims,
	}

	verifier := opts.New()

	if _, err := verifier.VerifyIdToken(token); err != nil {
		return err
	}

	return nil
}

func New(issuer string, claims map[string]string) auth.Provider {
	return &Provider{
		issuer: issuer,
		claims: claims,
	}
}
