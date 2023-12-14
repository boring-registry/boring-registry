package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/core"

	jwtverifier "github.com/okta/okta-jwt-verifier-golang/v2"
)

type OktaProvider struct {
	issuer string
	claims map[string]string
}

func (p *OktaProvider) String() string { return "okta" }

func (p *OktaProvider) Verify(ctx context.Context, token string) error {
	opts := jwtverifier.JwtVerifier{
		Issuer:           p.issuer,
		ClaimsToValidate: p.claims,
	}

	verifier, err := opts.New()
	if err != nil {
		return err
	}

	if _, err := verifier.VerifyIdToken(token); err != nil {
		return fmt.Errorf("%v: %w", core.ErrInvalidToken, err)
	}

	return nil
}

func NewOktaProvider(issuer string, claims ...string) Provider {
	m := make(map[string]string)

	for _, claim := range claims {
		parts := strings.Split(claim, "=")
		if len(parts) != 2 {
			continue
		}

		key, val := parts[0], parts[1]

		m[key] = val
	}

	return &OktaProvider{
		issuer: issuer,
		claims: m,
	}
}
