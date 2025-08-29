package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/audit"
	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func TestProviderListEndpointWithAudit(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockProviderService{
		providerVersions: &core.ProviderVersions{
			Versions: []core.ProviderVersion{
				{Namespace: "hashicorp", Name: "aws", Version: "5.0.0"},
				{Namespace: "hashicorp", Name: "aws", Version: "5.1.0"},
			},
		},
	}

	metrics := &o11y.ProviderMetrics{
		ListVersions: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_list_versions"},
			[]string{"namespace", "name"},
		),
	}

	endpoint := listEndpoint(mockService, metrics, testAuditLogger)

	user := &audit.UserContext{
		UserEmail: "test@example.com",
		UserName:  "Test User",
		Subject:   "user123",
	}
	ctx := audit.SetUserInContext(context.Background(), user)

	request := listRequest{
		namespace: "hashicorp",
		name:      "aws",
	}

	response, err := endpoint(ctx, request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	providerVersions, ok := response.(*core.ProviderVersions)
	if !ok {
		t.Fatal("Expected *core.ProviderVersions type")
	}

	if len(providerVersions.Versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(providerVersions.Versions))
	}

	if providerVersions.Versions[0].Namespace != "hashicorp" {
		t.Errorf("Expected namespace 'hashicorp', got %q", providerVersions.Versions[0].Namespace)
	}

	if len(testAuditLogger.events) != 1 {
		t.Errorf("Expected 1 audit event, got %d", len(testAuditLogger.events))
	}

	event := testAuditLogger.events[0]
	if event.Event != audit.EventRegistryProviderAccess {
		t.Errorf("Expected event %q, got %q", audit.EventRegistryProviderAccess, event.Event)
	}

	if event.Action != audit.ActionList {
		t.Errorf("Expected action %q, got %q", audit.ActionList, event.Action)
	}

	expectedResource := "hashicorp/aws"
	if event.Resource != expectedResource {
		t.Errorf("Expected resource %q, got %q", expectedResource, event.Resource)
	}

	if event.User == nil {
		t.Error("Expected user context in audit event")
	} else {
		if event.User.UserEmail != user.UserEmail {
			t.Errorf("Expected user email %q, got %q", user.UserEmail, event.User.UserEmail)
		}
	}

	if event.DurationMs < 0 {
		t.Error("Expected non-negative duration in audit event")
	}
}

func TestProviderDownloadEndpointWithAudit(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockProviderService{
		provider: &core.Provider{
			Namespace:           "hashicorp",
			Name:                "aws",
			Version:             "5.1.0",
			OS:                  "linux",
			Arch:                "amd64",
			Filename:            "terraform-provider-aws_5.1.0_linux_amd64.zip",
			DownloadURL:         "https://example.com/provider.zip",
			Shasum:              "abc123",
			SHASumsURL:          "https://example.com/shasums",
			SHASumsSignatureURL: "https://example.com/shasums.sig",
			SigningKeys: core.SigningKeys{
				GPGPublicKeys: []core.GPGPublicKey{
					{KeyID: "key123"},
				},
			},
		},
	}

	metrics := &o11y.ProviderMetrics{
		Download: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_download"},
			[]string{"namespace", "name", "version", "os", "arch"},
		),
	}

	endpoint := downloadEndpoint(mockService, metrics, testAuditLogger)

	user := &audit.UserContext{
		UserEmail: "test@example.com",
		UserName:  "Test User",
	}
	ctx := audit.SetUserInContext(context.Background(), user)

	request := downloadRequest{
		namespace: "hashicorp",
		name:      "aws",
		version:   "5.1.0",
		os:        "linux",
		arch:      "amd64",
	}

	response, err := endpoint(ctx, request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	downloadResp, ok := response.(downloadResponse)
	if !ok {
		t.Fatal("Expected downloadResponse type")
	}

	if downloadResp.DownloadURL != "https://example.com/provider.zip" {
		t.Errorf("Expected download URL 'https://example.com/provider.zip', got %q", downloadResp.DownloadURL)
	}

	if downloadResp.OS != "linux" {
		t.Errorf("Expected OS 'linux', got %q", downloadResp.OS)
	}

	if downloadResp.Arch != "amd64" {
		t.Errorf("Expected arch 'amd64', got %q", downloadResp.Arch)
	}

	if len(testAuditLogger.events) != 1 {
		t.Errorf("Expected 1 audit event, got %d", len(testAuditLogger.events))
	}

	event := testAuditLogger.events[0]
	if event.Event != audit.EventRegistryProviderAccess {
		t.Errorf("Expected event %q, got %q", audit.EventRegistryProviderAccess, event.Event)
	}

	if event.Action != audit.ActionDownload {
		t.Errorf("Expected action %q, got %q", audit.ActionDownload, event.Action)
	}

	expectedResource := "hashicorp/aws/5.1.0/linux/amd64"
	if event.Resource != expectedResource {
		t.Errorf("Expected resource %q, got %q", expectedResource, event.Resource)
	}

	if event.User == nil {
		t.Error("Expected user context in audit event")
	}
}

func TestProviderEndpointWithoutUserContext(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockProviderService{
		providerVersions: &core.ProviderVersions{
			Versions: []core.ProviderVersion{
				{Namespace: "hashicorp", Name: "aws", Version: "5.0.0"},
			},
		},
	}

	metrics := &o11y.ProviderMetrics{
		ListVersions: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_list_versions"},
			[]string{"namespace", "name"},
		),
	}

	endpoint := listEndpoint(mockService, metrics, testAuditLogger)

	ctx := context.Background()
	request := listRequest{
		namespace: "hashicorp",
		name:      "aws",
	}

	_, err := endpoint(ctx, request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(testAuditLogger.events) != 1 {
		t.Errorf("Expected 1 audit event, got %d", len(testAuditLogger.events))
	}

	event := testAuditLogger.events[0]
	if event.User != nil {
		t.Error("Expected no user context in audit event, got:", event.User)
	}
}

func TestProviderEndpointWithServiceError(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockProviderService{
		shouldError: true,
	}

	metrics := &o11y.ProviderMetrics{
		ListVersions: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_list_versions"},
			[]string{"namespace", "name"},
		),
	}

	endpoint := listEndpoint(mockService, metrics, testAuditLogger)

	ctx := context.Background()
	request := listRequest{
		namespace: "hashicorp",
		name:      "aws",
	}

	_, err := endpoint(ctx, request)
	if err == nil {
		t.Fatal("Expected error from service, got nil")
	}

	if len(testAuditLogger.events) != 0 {
		t.Errorf("Expected 0 audit events for error case, got %d", len(testAuditLogger.events))
	}
}

type testAuditLogger struct {
	events []*audit.AuditEvent
}

func (t *testAuditLogger) LogEvent(ctx context.Context, event *audit.AuditEvent) {
	t.events = append(t.events, event)
}

type mockProviderService struct {
	providerVersions *core.ProviderVersions
	provider         *core.Provider
	shouldError      bool
}

func (m *mockProviderService) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
	if m.shouldError {
		return nil, errors.New("service error")
	}
	return m.providerVersions, nil
}

func (m *mockProviderService) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	if m.shouldError {
		return nil, errors.New("service error")
	}
	return m.provider, nil
}
