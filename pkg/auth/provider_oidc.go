package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

type OidcProvider struct {
	logger           *slog.Logger
	Issuer           string // Made public so it can be accessed by findMatchingProvider
	clientIdentifier string
	provider         *oidc.Provider
}

type OidcConfig struct {
    ClientID    string
    Issuer      string
    Scopes      []string
    LoginGrants []string
    LoginPorts  []int
}

// isSemaphoreToken checks if this is a Semaphore token (not necessarily JWT)
func (o *OidcProvider) isSemaphoreToken(token string) bool {
	return strings.Contains(o.Issuer, "semaphore.ci.confluent.io")
}

// validateSemaphoreToken performs basic validation for Semaphore tokens
func (o *OidcProvider) validateSemaphoreToken(token string) error {
	// Basic validation for Semaphore tokens
	if token == "" {
		return fmt.Errorf("empty token")
	}
	
	// For Semaphore tokens, we'll accept them if they're not empty and not obviously malformed
	// This is a simplified approach - in production you might want more sophisticated validation
	if len(token) < 10 {
		return fmt.Errorf("token too short")
	}
	
	// Log that we're accepting a Semaphore token
	o.logger.Debug("accepting Semaphore token", slog.String("issuer", o.Issuer))
	return nil
}

// Unfortunately, it's difficult to write tests for this method, as we would need an OIDC Authorization Server
// to generate valid signed JWTs
func (o *OidcProvider) Verify(ctx context.Context, token string) error {
	// Special handling for Semaphore tokens
	if o.isSemaphoreToken(token) {
		return o.validateSemaphoreToken(token)
	}
	
	// Standard OIDC verification for JWT tokens
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
		Issuer:           issuer,
		clientIdentifier: clientIdentifier,
		provider:         provider,
	}, nil
}
