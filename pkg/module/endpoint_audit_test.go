package module

import (
	"context"
	"errors"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/audit"
	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func TestListEndpointWithAudit(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockModuleService{
		modules: []core.Module{
			{Namespace: "test", Name: "module", Provider: "aws", Version: "1.0.0"},
			{Namespace: "test", Name: "module", Provider: "aws", Version: "1.1.0"},
		},
	}

	metrics := &o11y.ModuleMetrics{
		ListVersions: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_list_versions"},
			[]string{"namespace", "name", "provider"},
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
		namespace: "test",
		name:      "module",
		provider:  "aws",
	}

	response, err := endpoint(ctx, request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	listResp, ok := response.(listResponse)
	if !ok {
		t.Fatal("Expected listResponse type")
	}

	if len(listResp.Modules) != 1 {
		t.Errorf("Expected 1 module in response, got %d", len(listResp.Modules))
	}

	if len(listResp.Modules[0].Versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(listResp.Modules[0].Versions))
	}

	if len(testAuditLogger.events) != 1 {
		t.Errorf("Expected 1 audit event, got %d", len(testAuditLogger.events))
	}

	event := testAuditLogger.events[0]
	if event.Event != audit.EventRegistryModuleAccess {
		t.Errorf("Expected event %q, got %q", audit.EventRegistryModuleAccess, event.Event)
	}

	if event.Action != audit.ActionList {
		t.Errorf("Expected action %q, got %q", audit.ActionList, event.Action)
	}

	expectedResource := "test/module/aws"
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

func TestDownloadEndpointWithAudit(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockModuleService{
		module: core.Module{
			Namespace:   "test",
			Name:        "module",
			Provider:    "aws",
			Version:     "1.0.0",
			DownloadURL: "https://example.com/module.tar.gz",
		},
	}

	metrics := &o11y.ModuleMetrics{
		Download: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_download"},
			[]string{"namespace", "name", "provider", "version"},
		),
	}

	endpoint := downloadEndpoint(mockService, metrics, testAuditLogger)

	user := &audit.UserContext{
		UserEmail: "test@example.com",
		UserName:  "Test User",
	}
	ctx := audit.SetUserInContext(context.Background(), user)

	request := downloadRequest{
		namespace: "test",
		name:      "module",
		provider:  "aws",
		version:   "1.0.0",
	}

	response, err := endpoint(ctx, request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	downloadResp, ok := response.(downloadResponse)
	if !ok {
		t.Fatal("Expected downloadResponse type")
	}

	expectedURL := "https://example.com/module.tar.gz"
	if downloadResp.url != expectedURL {
		t.Errorf("Expected URL %q, got %q", expectedURL, downloadResp.url)
	}

	if len(testAuditLogger.events) != 1 {
		t.Errorf("Expected 1 audit event, got %d", len(testAuditLogger.events))
	}

	event := testAuditLogger.events[0]
	if event.Event != audit.EventRegistryModuleAccess {
		t.Errorf("Expected event %q, got %q", audit.EventRegistryModuleAccess, event.Event)
	}

	if event.Action != audit.ActionDownload {
		t.Errorf("Expected action %q, got %q", audit.ActionDownload, event.Action)
	}

	expectedResource := "test/module/aws/1.0.0"
	if event.Resource != expectedResource {
		t.Errorf("Expected resource %q, got %q", expectedResource, event.Resource)
	}

	if event.User == nil {
		t.Error("Expected user context in audit event")
	}
}

func TestEndpointWithoutUserContext(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockModuleService{
		modules: []core.Module{
			{Namespace: "test", Name: "module", Provider: "aws", Version: "1.0.0"},
		},
	}

	metrics := &o11y.ModuleMetrics{
		ListVersions: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_list_versions"},
			[]string{"namespace", "name", "provider"},
		),
	}

	endpoint := listEndpoint(mockService, metrics, testAuditLogger)

	ctx := context.Background()
	request := listRequest{
		namespace: "test",
		name:      "module",
		provider:  "aws",
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

func TestEndpointWithServiceError(t *testing.T) {
	testAuditLogger := &testAuditLogger{events: make([]*audit.AuditEvent, 0)}

	mockService := &mockModuleService{
		shouldError: true,
	}

	metrics := &o11y.ModuleMetrics{
		ListVersions: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_list_versions"},
			[]string{"namespace", "name", "provider"},
		),
	}

	endpoint := listEndpoint(mockService, metrics, testAuditLogger)

	ctx := context.Background()
	request := listRequest{
		namespace: "test",
		name:      "module",
		provider:  "aws",
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

type mockModuleService struct {
	modules     []core.Module
	module      core.Module
	shouldError bool
}

func (m *mockModuleService) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	if m.shouldError {
		return nil, errors.New("service error")
	}
	return m.modules, nil
}

func (m *mockModuleService) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	if m.shouldError {
		return core.Module{}, errors.New("service error")
	}
	return m.module, nil
}
