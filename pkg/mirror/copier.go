package mirror

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
)

type Copier interface {
	// copy copies the artifacts of a provider to the pull-through cache/mirror
	copy(provider *core.Provider)
}

// copier implements Copier and ensures that requested providers are replicated to the internal storage asynchronously
type copier struct {
	// done is used to signal termination to potentially multiple goroutines at once
	done chan struct{}

	storage Storage
	client  *http.Client
	logger  *slog.Logger
}

// copy should be started in a separate goroutine
func (c *copier) copy(provider *core.Provider) {
	begin := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// A goroutine that terminates all pending downloads in case the application is shutting down
	go func() {
		select {
		case <-c.done:
			cancel()
		case <-ctx.Done():
			// No-op as the copy process either succeeded and the deferred cancel() function was called
			// or the operation timed out. In both cases, we just want to terminate the goroutine
		}
	}()

	// We download the files from upstream and mirror them to our storage
	if err := c.signingKeys(ctx, provider); err != nil {
		c.logger.Error("failed to copy signing keys", logKeyValues(provider), slog.String("err", err.Error()))
		return
	}

	if err := c.sha256Sums(ctx, provider); err != nil {
		c.logger.Error("failed to copy SHA256SUMS", logKeyValues(provider), slog.String("err", err.Error()))
		return
	}

	if err := c.sha256SumsSignature(ctx, provider); err != nil {
		c.logger.Error("failed to copy SHA256SUMS.sig", logKeyValues(provider), slog.String("err", err.Error()))
		return
	}

	// Request the provider archive
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.DownloadURL, nil)
	if err != nil {
		c.logger.Error("failed to create provider download request", logKeyValues(provider), slog.String("err", err.Error()))
		return
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("failed to download provider", logKeyValues(provider), slog.String("err", err.Error()))
		return
	}
	defer resp.Body.Close()

	fileName := provider.ArchiveFileName()
	if err = c.storage.UploadMirroredFile(ctx, provider, fileName, resp.Body); err != nil {
		c.logger.Error("failed to upload provider to mirror", logKeyValues(provider), slog.String("err", err.Error()))
	}
	c.logger.Info("successfully copied provider", logKeyValues(provider), slog.String("took", time.Since(begin).String()))
}

// check if the signing keys exist, if not add it
func (c *copier) signingKeys(ctx context.Context, provider *core.Provider) error {
	needsUpdate := true
	storedKeys, err := c.storage.MirroredSigningKeys(ctx, provider.Hostname, provider.Namespace)
	if err != nil {
		if !errors.Is(err, core.ErrObjectNotFound) {
			return err
		}

		// If the signing keys don't exist in the mirror, we override them with the upstream signing keys
		storedKeys = &provider.SigningKeys
	} else {
		storedKeys.GPGPublicKeys, needsUpdate = mergeGPGPublicKeys(provider.SigningKeys.GPGPublicKeys, storedKeys.GPGPublicKeys)
	}

	if !needsUpdate {
		return nil
	}

	return c.storage.UploadMirroredSigningKeys(ctx, provider.Hostname, provider.Namespace, storedKeys)
}

func (c *copier) sha256SumsSignature(ctx context.Context, provider *core.Provider) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.SHASumsSignatureURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download SHA256SUMS.sig, statuscode is %v", resp.StatusCode)
	}

	fileName := provider.ShasumSignatureFileName()
	return c.storage.UploadMirroredFile(ctx, provider, fileName, resp.Body)
}

func (c *copier) sha256Sums(ctx context.Context, provider *core.Provider) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.SHASumsURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download SHA256SUMS, statuscode is %v", resp.StatusCode)
	}

	fileName := provider.ShasumFileName()
	return c.storage.UploadMirroredFile(ctx, provider, fileName, resp.Body)
}

func (c *copier) shutdown(ctx context.Context) {
	<-ctx.Done()
	close(c.done)
}

func NewCopier(ctx context.Context, storage Storage) Copier {
	logger := slog.Default().With(slog.String("component", "copier"))
	m := &copier{
		done:   make(chan struct{}),
		logger: logger,
		client: &http.Client{
			// This is also the timeout for reading the response body
			Timeout: 2 * time.Minute,
		},
		storage: storage,
	}
	go m.shutdown(ctx)
	return m
}

func logKeyValues(provider *core.Provider) slog.Attr {
	return slog.Group("provider",
		slog.String("hostname", provider.Hostname),
		slog.String("namespace", provider.Namespace),
		slog.String("name", provider.Name),
		slog.String("version", provider.Version),
		slog.String("os", provider.OS),
		slog.String("arch", provider.Arch),
	)
}

func mergeGPGPublicKeys(upstreamKeys, mirroredKeys []core.GPGPublicKey) ([]core.GPGPublicKey, bool) {
	var merged []core.GPGPublicKey
	for _, upstreamKey := range upstreamKeys {
		for _, storedKey := range mirroredKeys {
			if storedKey.KeyID != upstreamKey.KeyID {
				merged = append(merged, upstreamKey)
			}
		}
	}

	return merged, len(merged) > len(upstreamKeys)
}
