package audit

import (
	"context"
	"fmt"
	"log/slog"
)

func CreateS3AuditLogger(ctx context.Context, s3Client S3ClientInterface, config Config) (Logger, error) {
	logger := slog.Default()
	
	if !config.Enabled {
		logger.Info("audit logging is disabled, using no-op logger")
		return &NoOpAuditLogger{}, nil
	}

	if s3Client == nil {
		return nil, fmt.Errorf("S3 client not available for S3 audit logging")
	}
	
	s3Config := config.GetS3Config()
	if s3Config.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket must be specified for S3 audit logging")
	}
	
	logger.Info("enabling S3 audit logging",
		slog.String("bucket", s3Config.Bucket),
		slog.String("prefix", s3Config.Prefix),
		slog.Int("batch_size", s3Config.BatchSize),
		slog.Duration("flush_interval", s3Config.FlushInterval))
	
	s3Logger, err := NewS3AuditLogger(s3Client, s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 audit logger: %w", err)
	}
	
	return s3Logger, nil
}

type NoOpAuditLogger struct{}

func (n *NoOpAuditLogger) LogEvent(ctx context.Context, event *AuditEvent) {
}
