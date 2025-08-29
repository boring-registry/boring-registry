package audit

import "time"

type Config struct {
	Enabled bool           `yaml:"enabled" json:"enabled"`
	S3      S3AuditConfig  `yaml:"s3" json:"s3"`
}

func DefaultConfig() Config {
	return Config{
		Enabled: true,
		S3: S3AuditConfig{
			BatchSize:     100,
			FlushInterval: 30 * time.Second,
			Prefix:        "audit-logs/",
		},
	}
}

func (c *Config) GetS3Config() S3AuditConfig {
	config := c.S3
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 30 * time.Second
	}
	if config.Prefix == "" {
		config.Prefix = "audit-logs/"
	}
	
	return config
}
