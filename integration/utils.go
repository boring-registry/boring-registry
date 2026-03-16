//go:build integration

package integration

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/boring-registry/boring-registry/pkg/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/azure/azurite"
)

var (
	letters = []rune("abcdefghijklmnopqrstuvwxyz")
)

type storageHarness struct {
	container   *azurite.Container
	client      *azblob.Client
	credentials *azblob.SharedKeyCredential
}

func (s *storageHarness) setupClient(ctx context.Context, t *testing.T) {
	cred, err := azblob.NewSharedKeyCredential(azurite.AccountName, azurite.AccountKey)
	if err != nil {
		t.Fatalf("failed to create shared key credential: %v", err)
	}

	serviceURL, err := s.container.BlobServiceURL(ctx)
	if err != nil {
		t.Fatalf("failed to get service URL: %v", err)
	}
	blobServiceURL := serviceURL + "/" + azurite.AccountName

	client, err := azblob.NewClientWithSharedKeyCredential(blobServiceURL, cred, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	s.client = client
	s.credentials = cred
}

func (s *storageHarness) createAzuriteContainer(ctx context.Context, t *testing.T) func() {
	azuriteContainer, err := azurite.Run(ctx,
		"mcr.microsoft.com/azure-storage/azurite:3.35.0",

		// The Azurite release schedule sometimes lags behind the release schedule of azure-sdk-for-go.
		// This can result in situations where the API schema in azure-sdk-for-go is more recent as azurite's.
		// THerefore we skip the API version check
		testcontainers.WithCmdArgs("--skipApiVersionCheck"),
	)
	if err != nil {
		t.Fatal(err)
	}

	s.container = azuriteContainer

	fn := func() {
		terminationCtx, terminationCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminationCancel()

		if err := azuriteContainer.Terminate(terminationCtx); err != nil {
			t.Fatalf("failed to terminate container: %v", err)
		}
	}
	return fn
}

func (s *storageHarness) newStorageInstance(ctx context.Context, t *testing.T) *storageInstance {
	instance := &storageInstance{
		storageHarness: s,
	}
	instance.container = instance.createContainer(ctx, t)

	return instance
}

type storageInstance struct {
	storageHarness *storageHarness
	container      string
}

func (s *storageInstance) createContainer(ctx context.Context, t *testing.T) string {
	length := 8
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	randomName := string(b)

	_, err := s.storageHarness.client.CreateContainer(ctx, randomName, nil)
	if err != nil {
		t.Fatalf("failed to create container %s: %v", randomName, err)
	}
	return randomName
}

func (s *storageInstance) setupStorage() storage.Storage {
	return storage.NewAzuriteStorage(s.storageHarness.client,
		s.storageHarness.credentials,
		azurite.AccountName,
		s.container,
		"",
		storage.DefaultModuleArchiveFormat,
		5*time.Minute,
	)
}

func (s *storageInstance) uploadedObjects(ctx context.Context, t *testing.T) []string {
	objects := []string{}
	pager := s.storageHarness.client.NewListBlobsFlatPager(s.container, nil)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			t.Fatalf("failed to list blobs: %v", err)
		}

		for _, i := range resp.Segment.BlobItems {
			objects = append(objects, *i.Name)
		}
	}

	return objects
}
