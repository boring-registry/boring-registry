package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Event types for audit logging
const (
	EventAuthLogin              = "auth.login"
	EventAuthLogout             = "auth.logout"
	EventAuthTokenValidation    = "auth.token_validation"
	EventRegistryModuleAccess   = "registry.module_access"
	EventRegistryProviderAccess = "registry.provider_access"
)

// Result types
const (
	ResultSuccess = "success"
	ResultFailed  = "failed"
)

// Action types
const (
	ActionDownload = "download"
	ActionList     = "list"
	ActionView     = "view"
)

// UserContext contains user information extracted from authentication
type UserContext struct {
	UserID    string `json:"user_id,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
	UserName  string `json:"user_name,omitempty"`
	Issuer    string `json:"issuer,omitempty"`
	Subject   string `json:"subject,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
}

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	Timestamp  time.Time    `json:"timestamp"`
	Level      string       `json:"level"`
	Event      string       `json:"event"`
	Result     string       `json:"result"`
	User       *UserContext `json:"user,omitempty"`
	SourceIP   string       `json:"source_ip,omitempty"`
	UserAgent  string       `json:"user_agent,omitempty"`
	Resource   string       `json:"resource,omitempty"`
	Action     string       `json:"action,omitempty"`
	DurationMs int64        `json:"duration_ms,omitempty"`
	Error      string       `json:"error,omitempty"`
	RequestID  string       `json:"request_id,omitempty"`
}

// Logger interface for audit logging
type Logger interface {
	LogEvent(ctx context.Context, event *AuditEvent)
}

// SlogAuditLogger implements audit logging using structured logging
type SlogAuditLogger struct {
	logger *slog.Logger
}

// NewSlogAuditLogger creates a new audit logger using slog
func NewSlogAuditLogger() *SlogAuditLogger {
	return &SlogAuditLogger{
		logger: slog.Default(),
	}
}

// LogEvent logs an audit event using structured logging
func (l *SlogAuditLogger) LogEvent(ctx context.Context, event *AuditEvent) {
	// Convert event to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		l.logger.Error("failed to marshal audit event", slog.String("err", err.Error()))
		return
	}

	// Log based on result
	if event.Result == ResultFailed {
		l.logger.Error("audit event", slog.String("audit_data", string(eventJSON)))
	} else {
		l.logger.Info("audit event", slog.String("audit_data", string(eventJSON)))
	}
}

// ExtractRequestInfo extracts common request information
func ExtractRequestInfo(r *http.Request) (sourceIP, userAgent, requestID string) {
	// Extract source IP (check for forwarded headers first)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		sourceIP = forwarded
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		sourceIP = realIP
	} else {
		sourceIP = r.RemoteAddr
	}

	// Extract user agent
	userAgent = r.Header.Get("User-Agent")

	// Extract or generate request ID
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		requestID = reqID
	} else if reqID := r.Header.Get("X-Correlation-ID"); reqID != "" {
		requestID = reqID
	}

	return sourceIP, userAgent, requestID
}

// Context keys for user information
type contextKey string

const (
	UserContextKey contextKey = "user_context"
)

// GetUserFromContext extracts user context from request context
func GetUserFromContext(ctx context.Context) *UserContext {
	if user, ok := ctx.Value(UserContextKey).(*UserContext); ok {
		return user
	}
	return nil
}

// SetUserInContext adds user context to request context
func SetUserInContext(ctx context.Context, user *UserContext) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// Helper functions for creating audit events

// LogAuthSuccess logs successful authentication
func LogAuthSuccess(ctx context.Context, logger Logger, user *UserContext, sourceIP, userAgent string, duration time.Duration) {
	event := &AuditEvent{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Event:      EventAuthLogin,
		Result:     ResultSuccess,
		User:       user,
		SourceIP:   sourceIP,
		UserAgent:  userAgent,
		DurationMs: duration.Milliseconds(),
	}
	logger.LogEvent(ctx, event)
}

// LogAuthFailure logs failed authentication
func LogAuthFailure(ctx context.Context, logger Logger, sourceIP, userAgent, errorMsg string, duration time.Duration) {
	event := &AuditEvent{
		Timestamp:  time.Now(),
		Level:      "ERROR",
		Event:      EventAuthLogin,
		Result:     ResultFailed,
		SourceIP:   sourceIP,
		UserAgent:  userAgent,
		Error:      errorMsg,
		DurationMs: duration.Milliseconds(),
	}
	logger.LogEvent(ctx, event)
}

// LogRegistryAccess logs registry resource access
func LogRegistryAccess(ctx context.Context, logger Logger, resourceType, resource, action string, duration time.Duration) {
	user := GetUserFromContext(ctx)

	var eventType string
	switch resourceType {
	case "module":
		eventType = EventRegistryModuleAccess
	case "provider":
		eventType = EventRegistryProviderAccess
	default:
		eventType = "registry.access"
	}

	event := &AuditEvent{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Event:      eventType,
		Result:     ResultSuccess,
		User:       user,
		Resource:   resource,
		Action:     action,
		DurationMs: duration.Milliseconds(),
	}
	logger.LogEvent(ctx, event)
}
