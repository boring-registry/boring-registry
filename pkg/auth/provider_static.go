package auth

import (
	"context"
	"strings"

	"github.com/TierMobility/boring-registry/pkg/core"
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

	return core.ErrInvalidToken
}

func NewStaticProvider(tokens ...string) Provider {
	// spf13/viper and spf13/pflag currently do not support reading multiple values from environment variables and
	// extracting them into a StringSlice/StringArray.
	// This workaround extracts comma-separated tokens into separate tokens
	//
	// See https://github.com/spf13/viper/issues/339 and https://github.com/spf13/viper/issues/380
	var parsed []string
	for _, t := range tokens {
		if strings.ContainsAny(t, ",") {
			split := strings.Split(t, ",")
			for _, s := range split {
				if s == "" {
					// Skip empty strings occurring due to splitting csv values like "test,"
					continue
				}
				parsed = append(parsed, s)
			}
		} else {
			parsed = append(parsed, t)
		}
	}

	return &StaticProvider{
		tokens: parsed,
	}
}
