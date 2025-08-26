package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const (
	EventAuthLogin              = "auth.login"
	EventAuthLogout             = "auth.logout"
	EventAuthTokenValidation    = "auth.token_validation"
	EventRegistryModuleAccess   = "registry.module_access"
	EventRegistryProviderAccess = "registry.provider_access"
)

const (
	ResultSuccess = "success"
	ResultFailed  = "failed"
)

const (
	ActionDownload = "download"
	ActionList     = "list"
	ActionView     = "view"
)

type UserContext struct {
	UserID    string `json:"user_id,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
	UserName  string `json:"user_name,omitempty"`
	Issuer    string `json:"issuer,omitempty"`
	Subject   string `json:"subject,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
}

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

type Logger interface {
	LogEvent(ctx context.Context, event *AuditEvent)
}

type SlogAuditLogger struct {
	logger *slog.Logger
}

func NewSlogAuditLogger() *SlogAuditLogger {
	return &SlogAuditLogger{
		logger: slog.Default(),
	}
}

func (l *SlogAuditLogger) LogEvent(ctx context.Context, event *AuditEvent) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		l.logger.Error("failed to marshal audit event", slog.String("err", err.Error()))
		return
	}

	if event.Result == ResultFailed {
		l.logger.Error("audit event", slog.String("audit_data", string(eventJSON)))
	} else {
		l.logger.Info("audit event", slog.String("audit_data", string(eventJSON)))
	}
}

func ExtractRequestInfo(r *http.Request) (sourceIP, userAgent, requestID string) {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		sourceIP = forwarded
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		sourceIP = realIP
	} else {
		sourceIP = r.RemoteAddr
	}

	userAgent = r.Header.Get("User-Agent")

	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		requestID = reqID
	} else if reqID := r.Header.Get("X-Correlation-ID"); reqID != "" {
		requestID = reqID
	}

	return sourceIP, userAgent, requestID
}

type contextKey string

const (
	UserContextKey contextKey = "user_context"
)

func GetUserFromContext(ctx context.Context) *UserContext {
	if user, ok := ctx.Value(UserContextKey).(*UserContext); ok {
		return user
	}
	return nil
}

func SetUserInContext(ctx context.Context, user *UserContext) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

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
