package static

import (
	"context"
	"fmt"

	"github.com/TierMobility/boring-registry/pkg/auth"
)

type Provider struct {
	token string
}

func (p *Provider) Verify(ctx context.Context, token string) error {
	if token == p.token {
		return nil
	}

	return fmt.Errorf("invalid token")
}

func New(token string) auth.Provider {
	return &Provider{
		token: token,
	}
}
