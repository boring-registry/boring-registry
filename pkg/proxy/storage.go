package proxy

import (
	"context"
)

// Storage represents the Storage of Terraform providers and modules.
type Storage interface {
	// Get a valid download URL from proxy link
	GetDownloadUrl(ctx context.Context, url string) (string, error)
}
