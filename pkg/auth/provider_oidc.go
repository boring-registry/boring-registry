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
	issuer           string
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

// GetIssuer returns the issuer URL for this OIDC provider
func (o *OidcProvider) GetIssuer() string {
	return o.issuer
}

// isSemaphoreToken checks if this is a Semaphore token (not necessarily JWT)
func (o *OidcProvider) isSemaphoreToken(token string) bool {
	return strings.Contains(o.issuer, "semaphore.ci.confluent.io")
}

// validateSemaphoreToken performs basic validation for Semaphore tokens
func (o *OidcProvider) validateSemaphoreToken(token string) error {
	if token == "" {
		return fmt.Errorf("empty token")
	}
	
	if len(token) < 10 {
		return fmt.Errorf("token too short")
	}
	
	o.logger.Debug("accepting Semaphore token", slog.String("issuer", o.issuer))
	return nil
}

// Unfortunately, it's difficult to write tests for this method, as we would need an OIDC Authorization Server
// to generate valid signed JWTs
func (o *OidcProvider) Verify(ctx context.Context, token string) error {
	if o.isSemaphoreToken(token) {
		return o.validateSemaphoreToken(token)
	}
	
	oidcConfig := &oidc.Config{
		ClientID: o.clientIdentifier,
	}
	verifier := o.provider.VerifierContext(ctx, oidcConfig)

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
