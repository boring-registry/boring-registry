package mirror

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/log"
)

type Copier interface {
	// copy copies the artifacts of a provider to the pull-through cache/mirror
	copy(provider *core.Provider)
}

// mirror implements Copier and ensures that requested providers are replicated to the internal storage asynchronously
type mirror struct {
	// done is used to signal termination to potentially multiple goroutines at once
	done chan struct{}

	storage Storage
	client  *http.Client
	logger  log.Logger
}

// copy should be started in a separate goroutine
func (m *mirror) copy(provider *core.Provider) {
	begin := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// A goroutine that terminates all pending downloads in case the application is shutting down
	go func() {
		select {
		case <-m.done:
			cancel()
		case <-ctx.Done():
			// No-op as the copy process either succeeded and the deferred cancel() function was called
			// or the operation timed out. In both cases, we just want to terminate the goroutine
		}
	}()

	// We download the files from upstream and mirror them to our storage
	if err := m.signingKeys(ctx, provider); err != nil {
		_ = m.logger.Log(logKeyValues(provider, err))
		return
	}

	if err := m.sha256Sums(ctx, provider); err != nil {
		_ = m.logger.Log(logKeyValues(provider, err))
		return
	}

	if err := m.sha256SumsSignature(ctx, provider); err != nil {
		_ = m.logger.Log(logKeyValues(provider, err))
		return
	}

	// Request the provider archive
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.DownloadURL, nil)
	if err != nil {
		_ = m.logger.Log(logKeyValues(provider, err))
		return
	}
	resp, err := m.client.Do(req)
	if err != nil {
		_ = m.logger.Log(logKeyValues(provider, err))
		return
	}
	defer resp.Body.Close()

	fileName := provider.ArchiveFileName()
	if err = m.storage.UploadMirroredFile(ctx, provider, fileName, resp.Body); err != nil {
		_ = m.logger.Log(logKeyValues(provider, err))
	}
	_ = m.logger.Log(
		"op", "CopyProvider",
		"hostname", provider.Hostname,
		"namespace", provider.Namespace,
		"name", provider.Name,
		"version", provider.Version,
		"os", provider.OS,
		"arch", provider.Arch,
		"took", time.Since(begin),
	)
}

// check if the signing keys exist, if not add it
func (m *mirror) signingKeys(ctx context.Context, provider *core.Provider) error {
	needsUpdate := true
	storedKeys, err := m.storage.MirroredSigningKeys(ctx, provider.Hostname, provider.Namespace)
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

	return m.storage.UploadMirroredSigningKeys(ctx, provider.Hostname, provider.Namespace, storedKeys)
}

func (m *mirror) sha256SumsSignature(ctx context.Context, provider *core.Provider) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.SHASumsSignatureURL, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download SHA256SUMS.sig, statuscode is %v", resp.StatusCode)
	}

	fileName := provider.ShasumSignatureFileName()
	return m.storage.UploadMirroredFile(ctx, provider, fileName, resp.Body)
}

func (m *mirror) sha256Sums(ctx context.Context, provider *core.Provider) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.SHASumsURL, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download SHA256SUMS, statuscode is %v", resp.StatusCode)
	}

	fileName := provider.ShasumFileName()
	return m.storage.UploadMirroredFile(ctx, provider, fileName, resp.Body)
}

func (m *mirror) shutdown(ctx context.Context) {
	<-ctx.Done()
	close(m.done)
}

func NewMirror(ctx context.Context, logger log.Logger, storage Storage) Copier {
	m := &mirror{
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

func logKeyValues(provider *core.Provider, err error) []string {
	return []string{
		"op", "CopyProvider",
		"hostname", provider.Hostname,
		"namespace", provider.Namespace,
		"name", provider.Name,
		"version", provider.Version,
		"os", provider.OS,
		"arch", provider.Arch,
		"err", err.Error(),
	}
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
