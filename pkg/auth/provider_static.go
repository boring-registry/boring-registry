package auth

import (
	"context"
)

type StaticProvider struct {
	tokens []string
}

func (p *StaticProvider) String() string { return "static" }

func (p *StaticProvider) Verify(ctx context.Context, token string) error {
	for _, validToken := range p.tokens {
		if token == validToken {
			return nil
		}
	}

	return ErrInvalidToken
}

func NewStaticProvider(tokens ...string) Provider {
	return &StaticProvider{
		tokens: tokens,
	}
}
