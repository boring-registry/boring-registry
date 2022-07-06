package auth

import (
	"context"

	jwtverifier "github.com/okta/okta-jwt-verifier-golang"
	"github.com/pkg/errors"
)

type OktaProvider struct {
	issuer string
	claims map[string]string
}

func (p *OktaProvider) Verify(ctx context.Context, token string) error {
	opts := jwtverifier.JwtVerifier{
		Issuer:           p.issuer,
		ClaimsToValidate: p.claims,
	}

	verifier := opts.New()

	if _, err := verifier.VerifyIdToken(token); err != nil {
		return errors.Wrap(ErrInvalidToken, err.Error())
	}

	return nil
}

func NewOktaProvider(issuer string, claims map[string]string) Provider {
	return &OktaProvider{
		issuer: issuer,
		claims: claims,
	}
}
