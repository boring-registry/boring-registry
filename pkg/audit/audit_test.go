package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestNewSlogAuditLogger(t *testing.T) {
	logger := NewSlogAuditLogger()
	if logger == nil {
		t.Fatal("NewSlogAuditLogger returned nil")
	}
}

func TestSlogAuditLogger_LogEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := &SlogAuditLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, nil)),
	}

	tests := []struct {
		name     string
		event    *AuditEvent
		wantErr  bool
		contains []string
	}{
		{
			name: "successful event",
			event: &AuditEvent{
				Timestamp: time.Now(),
				Level:     "INFO",
				Event:     EventAuthLogin,
				Result:    ResultSuccess,
				User: &UserContext{
					UserEmail: "test@example.com",
					UserName:  "Test User",
					Subject:   "user123",
				},
				SourceIP:  "192.168.1.1",
				UserAgent: "TestAgent/1.0",
			},
			contains: []string{"auth.login", "success", "test@example.com"},
		},
		{
			name: "failed event",
			event: &AuditEvent{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Event:     EventAuthLogin,
				Result:    ResultFailed,
				Error:     "invalid credentials",
				SourceIP:  "192.168.1.1",
				UserAgent: "TestAgent/1.0",
			},
			contains: []string{"auth.login", "failed", "invalid credentials"},
		},
		{
			name: "registry access event",
			event: &AuditEvent{
				Timestamp: time.Now(),
				Level:     "INFO",
				Event:     EventRegistryModuleAccess,
				Result:    ResultSuccess,
				User: &UserContext{
					UserEmail: "user@example.com",
					UserName:  "User Name",
				},
				Resource:   "aws/vpc/1.0.0",
				Action:     ActionDownload,
				DurationMs: 150,
			},
			contains: []string{"registry.module_access", "aws/vpc/1.0.0", "download"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			logger.LogEvent(context.Background(), tt.event)

			output := buf.String()
			if output == "" {
				t.Fatal("No output from logger")
			}

			for _, expected := range tt.contains {
				if !bytes.Contains(buf.Bytes(), []byte(expected)) {
					t.Errorf("Expected output to contain %q, got: %s", expected, output)
				}
			}
		})
	}
}

func TestUserContextInContext(t *testing.T) {
	ctx := context.Background()
	user := &UserContext{
		UserID:    "user123",
		UserEmail: "test@example.com",
		UserName:  "Test User",
		Subject:   "sub123",
		Issuer:    "https://auth.example.com",
		ClientID:  "client123",
	}

	ctxWithUser := SetUserInContext(ctx, user)
	if ctxWithUser == ctx {
		t.Error("SetUserInContext should return a new context")
	}

	retrievedUser := GetUserFromContext(ctxWithUser)
	if retrievedUser == nil {
		t.Fatal("GetUserFromContext returned nil")
	}

	if retrievedUser.UserID != user.UserID {
		t.Errorf("Expected UserID %q, got %q", user.UserID, retrievedUser.UserID)
	}
	if retrievedUser.UserEmail != user.UserEmail {
		t.Errorf("Expected UserEmail %q, got %q", user.UserEmail, retrievedUser.UserEmail)
	}
	if retrievedUser.UserName != user.UserName {
		t.Errorf("Expected UserName %q, got %q", user.UserName, retrievedUser.UserName)
	}

	emptyUser := GetUserFromContext(ctx)
	if emptyUser != nil {
		t.Error("GetUserFromContext should return nil for context without user")
	}
}

func TestExtractRequestInfo(t *testing.T) {
	tests := []struct {
		name          string
		headers       map[string]string
		remoteAddr    string
		expectedIP    string
		expectedUA    string
		expectedReqID string
	}{
		{
			name: "basic request",
			headers: map[string]string{
				"User-Agent": "TestAgent/1.0",
			},
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "192.168.1.1:12345",
			expectedUA:    "TestAgent/1.0",
			expectedReqID: "",
		},
		{
			name: "with X-Forwarded-For",
			headers: map[string]string{
				"User-Agent":      "TestAgent/1.0",
				"X-Forwarded-For": "203.0.113.1",
				"X-Request-ID":    "req-123",
			},
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "203.0.113.1",
			expectedUA:    "TestAgent/1.0",
			expectedReqID: "req-123",
		},
		{
			name: "with X-Real-IP",
			headers: map[string]string{
				"User-Agent":       "TestAgent/1.0",
				"X-Real-IP":        "198.51.100.1",
				"X-Correlation-ID": "corr-456",
			},
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "198.51.100.1",
			expectedUA:    "TestAgent/1.0",
			expectedReqID: "corr-456",
		},
		{
			name: "X-Forwarded-For takes precedence",
			headers: map[string]string{
				"User-Agent":      "TestAgent/1.0",
				"X-Forwarded-For": "203.0.113.1",
				"X-Real-IP":       "198.51.100.1",
			},
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "203.0.113.1",
			expectedUA:    "TestAgent/1.0",
			expectedReqID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			sourceIP, userAgent, requestID := ExtractRequestInfo(req)

			if sourceIP != tt.expectedIP {
				t.Errorf("Expected sourceIP %q, got %q", tt.expectedIP, sourceIP)
			}
			if userAgent != tt.expectedUA {
				t.Errorf("Expected userAgent %q, got %q", tt.expectedUA, userAgent)
			}
			if requestID != tt.expectedReqID {
				t.Errorf("Expected requestID %q, got %q", tt.expectedReqID, requestID)
			}
		})
	}
}

func TestLogAuthSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := &SlogAuditLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, nil)),
	}

	user := &UserContext{
		UserEmail: "test@example.com",
		UserName:  "Test User",
		Subject:   "user123",
	}

	LogAuthSuccess(context.Background(), logger, user, "192.168.1.1", "TestAgent/1.0", 100*time.Millisecond)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatal("Failed to parse log output as JSON:", err)
	}

	auditData, ok := logEntry["audit_data"].(string)
	if !ok {
		t.Fatal("Missing audit_data field")
	}

	var auditEvent AuditEvent
	if err := json.Unmarshal([]byte(auditData), &auditEvent); err != nil {
		t.Fatal("Failed to parse audit_data:", err)
	}

	if auditEvent.Event != EventAuthLogin {
		t.Errorf("Expected event %q, got %q", EventAuthLogin, auditEvent.Event)
	}
	if auditEvent.Result != ResultSuccess {
		t.Errorf("Expected result %q, got %q", ResultSuccess, auditEvent.Result)
	}
	if auditEvent.Level != "INFO" {
		t.Errorf("Expected level %q, got %q", "INFO", auditEvent.Level)
	}
	if auditEvent.DurationMs != 100 {
		t.Errorf("Expected duration 100ms, got %d", auditEvent.DurationMs)
	}
}

func TestLogAuthFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := &SlogAuditLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, nil)),
	}

	LogAuthFailure(context.Background(), logger, "192.168.1.1", "TestAgent/1.0", "invalid token", 50*time.Millisecond)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatal("Failed to parse log output as JSON:", err)
	}

	auditData, ok := logEntry["audit_data"].(string)
	if !ok {
		t.Fatal("Missing audit_data field")
	}

	var auditEvent AuditEvent
	if err := json.Unmarshal([]byte(auditData), &auditEvent); err != nil {
		t.Fatal("Failed to parse audit_data:", err)
	}

	if auditEvent.Event != EventAuthLogin {
		t.Errorf("Expected event %q, got %q", EventAuthLogin, auditEvent.Event)
	}
	if auditEvent.Result != ResultFailed {
		t.Errorf("Expected result %q, got %q", ResultFailed, auditEvent.Result)
	}
	if auditEvent.Level != "ERROR" {
		t.Errorf("Expected level %q, got %q", "ERROR", auditEvent.Level)
	}
	if auditEvent.Error != "invalid token" {
		t.Errorf("Expected error %q, got %q", "invalid token", auditEvent.Error)
	}
}

func TestLogRegistryAccess(t *testing.T) {
	var buf bytes.Buffer
	logger := &SlogAuditLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, nil)),
	}

	user := &UserContext{
		UserEmail: "test@example.com",
		UserName:  "Test User",
	}
	ctx := SetUserInContext(context.Background(), user)

	tests := []struct {
		name          string
		resourceType  string
		resource      string
		action        string
		expectedEvent string
	}{
		{
			name:          "module access",
			resourceType:  "module",
			resource:      "aws/vpc/1.0.0",
			action:        ActionDownload,
			expectedEvent: EventRegistryModuleAccess,
		},
		{
			name:          "provider access",
			resourceType:  "provider",
			resource:      "hashicorp/aws/5.1.0",
			action:        ActionList,
			expectedEvent: EventRegistryProviderAccess,
		},
		{
			name:          "unknown resource type",
			resourceType:  "unknown",
			resource:      "some/resource",
			action:        ActionView,
			expectedEvent: "registry.access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			LogRegistryAccess(ctx, logger, tt.resourceType, tt.resource, tt.action, 200*time.Millisecond)

			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatal("Failed to parse log output as JSON:", err)
			}

			auditData, ok := logEntry["audit_data"].(string)
			if !ok {
				t.Fatal("Missing audit_data field")
			}

			var auditEvent AuditEvent
			if err := json.Unmarshal([]byte(auditData), &auditEvent); err != nil {
				t.Fatal("Failed to parse audit_data:", err)
			}

			if auditEvent.Event != tt.expectedEvent {
				t.Errorf("Expected event %q, got %q", tt.expectedEvent, auditEvent.Event)
			}
			if auditEvent.Resource != tt.resource {
				t.Errorf("Expected resource %q, got %q", tt.resource, auditEvent.Resource)
			}
			if auditEvent.Action != tt.action {
				t.Errorf("Expected action %q, got %q", tt.action, auditEvent.Action)
			}
			if auditEvent.DurationMs != 200 {
				t.Errorf("Expected duration 200ms, got %d", auditEvent.DurationMs)
			}
		})
	}
}

func TestLogRegistryAccessWithoutUser(t *testing.T) {
	var buf bytes.Buffer
	logger := &SlogAuditLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, nil)),
	}

	ctx := context.Background()
	LogRegistryAccess(ctx, logger, "module", "aws/vpc/1.0.0", ActionDownload, 100*time.Millisecond)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatal("Failed to parse log output as JSON:", err)
	}

	auditData, ok := logEntry["audit_data"].(string)
	if !ok {
		t.Fatal("Missing audit_data field")
	}

	var auditEvent AuditEvent
	if err := json.Unmarshal([]byte(auditData), &auditEvent); err != nil {
		t.Fatal("Failed to parse audit_data:", err)
	}

	if auditEvent.User != nil {
		t.Error("Expected no user context, but got:", auditEvent.User)
	}
}

func TestConfig_GetS3Config(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected S3AuditConfig
	}{
		{
			name: "config with all values set",
			config: Config{
				Enabled: true,
				S3: S3AuditConfig{
					Bucket:        "test-bucket",
					Region:        "us-west-2",
					Prefix:        "custom-prefix/",
					BatchSize:     50,
					FlushInterval: 15 * time.Second,
				},
			},
			expected: S3AuditConfig{
				Bucket:        "test-bucket",
				Region:        "us-west-2",
				Prefix:        "custom-prefix/",
				BatchSize:     50,
				FlushInterval: 15 * time.Second,
			},
		},
		{
			name: "config with defaults applied",
			config: Config{
				Enabled: true,
				S3: S3AuditConfig{
					Bucket: "test-bucket",
					Region: "us-east-1",
				},
			},
			expected: S3AuditConfig{
				Bucket:        "test-bucket",
				Region:        "us-east-1",
				Prefix:        "audit-logs/",
				BatchSize:     100,
				FlushInterval: 30 * time.Second,
			},
		},
		{
			name: "config with zero batch size gets default",
			config: Config{
				Enabled: true,
				S3: S3AuditConfig{
					Bucket:    "test-bucket",
					Region:    "us-east-1",
					BatchSize: 0,
				},
			},
			expected: S3AuditConfig{
				Bucket:        "test-bucket",
				Region:        "us-east-1",
				Prefix:        "audit-logs/",
				BatchSize:     100,
				FlushInterval: 30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetS3Config()
			
			if result.Bucket != tt.expected.Bucket {
				t.Errorf("Expected bucket %q, got %q", tt.expected.Bucket, result.Bucket)
			}
			if result.Region != tt.expected.Region {
				t.Errorf("Expected region %q, got %q", tt.expected.Region, result.Region)
			}
			if result.Prefix != tt.expected.Prefix {
				t.Errorf("Expected prefix %q, got %q", tt.expected.Prefix, result.Prefix)
			}
			if result.BatchSize != tt.expected.BatchSize {
				t.Errorf("Expected batch size %d, got %d", tt.expected.BatchSize, result.BatchSize)
			}
			if result.FlushInterval != tt.expected.FlushInterval {
				t.Errorf("Expected flush interval %v, got %v", tt.expected.FlushInterval, result.FlushInterval)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if !config.Enabled {
		t.Error("Expected audit to be enabled by default")
	}
	
	s3Config := config.GetS3Config()
	if s3Config.BatchSize != 100 {
		t.Errorf("Expected default batch size 100, got %d", s3Config.BatchSize)
	}
	
	if s3Config.FlushInterval != 30*time.Second {
		t.Errorf("Expected default flush interval 30s, got %v", s3Config.FlushInterval)
	}
	
	if s3Config.Prefix != "audit-logs/" {
		t.Errorf("Expected default prefix 'audit-logs/', got %q", s3Config.Prefix)
	}
}

func TestNoOpAuditLogger(t *testing.T) {
	logger := &NoOpAuditLogger{}
	
	event := &AuditEvent{
		Timestamp: time.Now(),
		Level:     "INFO",
		Event:     EventAuthLogin,
		Result:    ResultSuccess,
	}
	
	logger.LogEvent(context.Background(), event)
}

type mockS3Client struct {
	putObjectCalls []s3.PutObjectInput
	shouldError    bool
	errorMessage   string
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.shouldError {
		return nil, fmt.Errorf("%s", m.errorMessage)
	}
	
	m.putObjectCalls = append(m.putObjectCalls, *params)
	
	return &s3.PutObjectOutput{}, nil
}

func TestCreateS3AuditLogger(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		s3Client    S3ClientInterface
		expectError bool
		errorMsg    string
	}{
		{
			name: "audit disabled returns no-op logger",
			config: Config{
				Enabled: false,
				S3: S3AuditConfig{
					Bucket: "test-bucket",
					Region: "us-west-2",
				},
			},
			s3Client:    &mockS3Client{},
			expectError: false,
		},
		{
			name: "valid config with S3 client",
			config: Config{
				Enabled: true,
				S3: S3AuditConfig{
					Bucket: "test-bucket",
					Region: "us-west-2",
				},
			},
			s3Client:    &mockS3Client{},
			expectError: false,
		},
		{
			name: "missing S3 client returns error",
			config: Config{
				Enabled: true,
				S3: S3AuditConfig{
					Bucket: "test-bucket",
					Region: "us-west-2",
				},
			},
			s3Client:    nil,
			expectError: true,
			errorMsg:    "S3 client not available",
		},
		{
			name: "missing bucket returns error",
			config: Config{
				Enabled: true,
				S3: S3AuditConfig{
					Region: "us-west-2",
				},
			},
			s3Client:    &mockS3Client{},
			expectError: true,
			errorMsg:    "S3 bucket must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := CreateS3AuditLogger(context.Background(), tt.s3Client, tt.config)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if logger == nil {
				t.Error("Expected logger but got nil")
			}
			
			if !tt.config.Enabled {
				_, isNoOp := logger.(*NoOpAuditLogger)
				if !isNoOp {
					t.Error("Expected NoOpAuditLogger for disabled audit")
				}
			}
		})
	}
}

func TestS3AuditLogger_Basic(t *testing.T) {
	mockClient := &mockS3Client{}
	
	config := S3AuditConfig{
		Bucket:        "test-audit-bucket",
		Region:        "us-west-2",
		Prefix:        "audit-logs/",
		BatchSize:     2,
		FlushInterval: 100 * time.Millisecond,
	}
	
	logger, err := NewS3AuditLogger(mockClient, config)
	if err != nil {
		t.Fatalf("Failed to create S3 audit logger: %v", err)
	}
	defer logger.Close()
	
	// Log a single event
	event := &AuditEvent{
		Timestamp: time.Now(),
		Level:     "INFO",
		Event:     EventAuthLogin,
		Result:    ResultSuccess,
		User: &UserContext{
			UserEmail: "test@example.com",
			UserName:  "Test User",
		},
		SourceIP:  "192.168.1.1",
		UserAgent: "TestAgent/1.0",
	}
	
	logger.LogEvent(context.Background(), event)
	
	event2 := &AuditEvent{
		Timestamp: time.Now(),
		Level:     "INFO",
		Event:     EventRegistryModuleAccess,
		Result:    ResultSuccess,
		Resource:  "test/module/aws/1.0.0",
		Action:    ActionDownload,
	}
	
	logger.LogEvent(context.Background(), event2)
	
	time.Sleep(200 * time.Millisecond)
	if len(mockClient.putObjectCalls) != 1 {
		t.Errorf("Expected 1 S3 PutObject call, got %d", len(mockClient.putObjectCalls))
	}
	
	if len(mockClient.putObjectCalls) > 0 {
		call := mockClient.putObjectCalls[0]
		
		if *call.Bucket != "test-audit-bucket" {
			t.Errorf("Expected bucket 'test-audit-bucket', got %q", *call.Bucket)
		}
		
		if !strings.Contains(*call.Key, "audit-logs/") {
			t.Errorf("Expected key to contain 'audit-logs/', got %q", *call.Key)
		}
		
		if !strings.Contains(*call.Key, "year=") {
			t.Errorf("Expected key to contain partitioning, got %q", *call.Key)
		}
		
		if *call.ContentType != "application/json" {
			t.Errorf("Expected content type 'application/json', got %q", *call.ContentType)
		}
		
		eventCount, exists := call.Metadata["event-count"]
		if !exists {
			t.Error("Expected 'event-count' in metadata")
		} else if eventCount != "2" {
			t.Errorf("Expected event count '2', got %q", eventCount)
		}
	}
}

func TestS3AuditLogger_TimeBasedFlush(t *testing.T) {
	mockClient := &mockS3Client{}
	
	config := S3AuditConfig{
		Bucket:        "test-audit-bucket",
		Region:        "us-west-2",
		Prefix:        "audit-logs/",
		BatchSize:     100,
		FlushInterval: 50 * time.Millisecond,
	}
	
	logger, err := NewS3AuditLogger(mockClient, config)
	if err != nil {
		t.Fatalf("Failed to create S3 audit logger: %v", err)
	}
	defer logger.Close()
	
	event := &AuditEvent{
		Timestamp: time.Now(),
		Level:     "INFO",
		Event:     EventAuthLogin,
		Result:    ResultSuccess,
	}
	
	logger.LogEvent(context.Background(), event)
	
	time.Sleep(100 * time.Millisecond)
	if len(mockClient.putObjectCalls) != 1 {
		t.Errorf("Expected 1 S3 PutObject call from time-based flush, got %d", len(mockClient.putObjectCalls))
	}
}
