package auth

import (
	"context"
	"slices"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/core"
)

type StaticProvider struct {
	tokens []string
}

func (p *StaticProvider) String() string { return "static" }

func (p *StaticProvider) Verify(_ context.Context, token string) error {
	if slices.Contains(p.tokens, token) {
		return nil
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
			for s := range strings.SplitSeq(t, ",") {
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
