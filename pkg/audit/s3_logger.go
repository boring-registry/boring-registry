package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3ClientInterface interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type S3AuditLogger struct {
	s3Client     S3ClientInterface
	bucket       string
	prefix       string
	batchSize    int
	flushInterval time.Duration
	logger       *slog.Logger
	
	eventBuffer  []*AuditEvent
	bufferMutex  sync.Mutex
	lastFlush    time.Time
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

type S3AuditConfig struct {
	Bucket        string        `yaml:"bucket" json:"bucket"`
	Region        string        `yaml:"region" json:"region"`
	Prefix        string        `yaml:"prefix" json:"prefix"`
	BatchSize     int           `yaml:"batch_size" json:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
}

func NewS3AuditLogger(s3Client S3ClientInterface, config S3AuditConfig) (*S3AuditLogger, error) {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 30 * time.Second
	}
	if config.Prefix == "" {
		config.Prefix = "audit-logs/"
	}

	logger := &S3AuditLogger{
		s3Client:      s3Client,
		bucket:        config.Bucket,
		prefix:        config.Prefix,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		logger:        slog.Default(),
		eventBuffer:   make([]*AuditEvent, 0, config.BatchSize),
		lastFlush:     time.Now(),
		stopChan:      make(chan struct{}),
	}

	logger.wg.Add(1)
	go logger.flushRoutine()

	return logger, nil
}

func (l *S3AuditLogger) LogEvent(ctx context.Context, event *AuditEvent) {
	l.bufferMutex.Lock()
	defer l.bufferMutex.Unlock()

	l.eventBuffer = append(l.eventBuffer, event)

	if len(l.eventBuffer) >= l.batchSize {
		l.flushBufferUnsafe(ctx)
	}
}

func (l *S3AuditLogger) flushRoutine() {
	defer l.wg.Done()
	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.bufferMutex.Lock()
			if len(l.eventBuffer) > 0 && time.Since(l.lastFlush) >= l.flushInterval {
				l.flushBufferUnsafe(context.Background())
			}
			l.bufferMutex.Unlock()
		case <-l.stopChan:
			l.bufferMutex.Lock()
			if len(l.eventBuffer) > 0 {
				l.flushBufferUnsafe(context.Background())
			}
			l.bufferMutex.Unlock()
			return
		}
	}
}

func (l *S3AuditLogger) flushBufferUnsafe(ctx context.Context) {
	if len(l.eventBuffer) == 0 {
		return
	}

	eventsToFlush := make([]*AuditEvent, len(l.eventBuffer))
	copy(eventsToFlush, l.eventBuffer)
	
	l.eventBuffer = l.eventBuffer[:0]
	l.lastFlush = time.Now()

	l.bufferMutex.Unlock()
	defer l.bufferMutex.Lock()
	batchData := struct {
		Events    []*AuditEvent `json:"events"`
		BatchInfo struct {
			Count     int       `json:"count"`
			Timestamp time.Time `json:"timestamp"`
		} `json:"batch_info"`
	}{
		Events: eventsToFlush,
	}
	batchData.BatchInfo.Count = len(eventsToFlush)
	batchData.BatchInfo.Timestamp = time.Now()

	jsonData, err := json.Marshal(batchData)
	if err != nil {
		l.logger.Error("failed to marshal audit events", slog.String("err", err.Error()))
		return
	}

	now := time.Now().UTC()
	key := fmt.Sprintf("%syear=%d/month=%02d/day=%02d/hour=%02d/audit-events-%d-%03d.json",
		l.prefix,
		now.Year(), now.Month(), now.Day(), now.Hour(),
		now.Unix(),
		len(eventsToFlush))
	_, err = l.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(l.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
		Metadata: map[string]string{
			"event-count": fmt.Sprintf("%d", len(eventsToFlush)),
			"created-at":  now.Format(time.RFC3339),
		},
	})

	if err != nil {
		l.logger.Error("failed to upload audit events to S3",
			slog.String("bucket", l.bucket),
			slog.String("key", key),
			slog.String("err", err.Error()))
		return
	}

	l.logger.Debug("successfully uploaded audit events to S3",
		slog.String("key", key),
		slog.Int("event_count", len(eventsToFlush)))
}

func (l *S3AuditLogger) Close() error {
	close(l.stopChan)
	l.wg.Wait()
	return nil
}

func (l *S3AuditLogger) Flush(ctx context.Context) error {
	l.bufferMutex.Lock()
	defer l.bufferMutex.Unlock()
	l.flushBufferUnsafe(ctx)
	return nil
}
