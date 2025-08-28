# Introduction

## Configuration

The boring-registry does not rely on a configuration file.
Instead, everything can be configured using command line flags or environment variables.

Important Note:

- Flags have higher priority than environment variables
- All environment variables are prefixed with `BORING_REGISTRY_`

Example: To enable debug logging you can either pass the `--debug` flag or set the environment `BORING_REGISTRY_DEBUG=true` variable.

## Authentication

- [API token](./authentication/api-token.md)
- [OIDC](./authentication/oidc.md)
- [Okta](./authentication/okta.md)

## Storage Backends

The boring-registry persists modules and providers in an object storage.
More information about the supported object storage solutions can be found here:

- [AWS S3](./storage-backends/aws-s3.md)
- [Azure Blob Storage](./storage-backends/azure-blob-storage.md)
- [Google Cloud Storage](./storage-backends/google-cloud-storage.md)
- [MinIO](./storage-backends/minio.md)

## Audit Logging

The boring-registry can audit user activities and registry access events to AWS S3 for compliance and monitoring purposes.

### Overview

Audit logging captures structured events including:

- Authentication success/failure events
- Registry access (module and provider downloads)
- User context (email, subject, IP address, user agent)
- Request metadata (method, path, timestamp)

Events are batched and written to S3 with time-based partitioning (`year/month/day/hour`) for efficient querying and storage.

### Authorization

Make sure the boring-registry has valid AWS credentials set which are authorized to access the S3 audit bucket.
This can for example be achieved by setting the `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables.

The service requires the following S3 permissions:
- `s3:PutObject`
- `s3:PutObjectAcl`
- `s3:ListBucket`

More information on this topic can be found in the [official documentation by AWS](https://docs.aws.amazon.com/sdkref/latest/guide/creds-config-files.html).

### Configuration

The following configuration options are available:

|Flag|Environment Variable|Description|
|---|---|---|
|`--audit-enabled`|`BORING_REGISTRY_AUDIT_ENABLED`|Enable S3 audit logging (default: true)|
|`--audit-s3-bucket`|`BORING_REGISTRY_AUDIT_S3_BUCKET`|S3 bucket to store audit logs|
|`--audit-s3-region`|`BORING_REGISTRY_AUDIT_S3_REGION`|S3 region for audit logs (defaults to storage S3 region)|
|`--audit-s3-prefix`|`BORING_REGISTRY_AUDIT_S3_PREFIX`|S3 prefix for audit logs (default: "audit-logs/")|
|`--audit-s3-batch-size`|`BORING_REGISTRY_AUDIT_S3_BATCH_SIZE`|Batch size for S3 audit uploads (default: 100)|
|`--audit-s3-flush-interval`|`BORING_REGISTRY_AUDIT_S3_FLUSH_INTERVAL`|Flush interval for S3 audit uploads (default: 30s)|

### Example

The following shows a minimal example to run `boring-registry server` with audit logging:

```console
$ boring-registry server \
  --storage-s3-bucket=boring-registry \
  --storage-s3-region=us-east-1 \
  --audit-enabled=true \
  --audit-s3-bucket=boring-registry-audit \
  --audit-s3-region=us-east-1
```

### S3 Object Structure

Audit logs are stored in S3 with the following structure:

```
audit-logs/
├── year=2024/
│   └── month=12/
│       └── day=19/
│           └── hour=15/
│               ├── audit-20241219-150000-001.json
│               ├── audit-20241219-150030-002.json
│               └── ...
```

Each file contains a JSON array of audit events:

```json
[
  {
    "timestamp": "2024-12-19T15:00:00Z",
    "event_type": "auth_success",
    "user_email": "user@example.com",
    "user_subject": "user123",
    "ip_address": "192.168.1.100",
    "user_agent": "terraform/1.6.0",
    "request_method": "GET",
    "request_path": "/v1/modules",
    "details": {}
  }
]
```

### Helm Configuration

For Helm deployments, add the audit configuration to your `values.yaml`:

```yaml
server:
  audit:
    enabled: true
    s3:
      bucket: "boring-registry-audit"
      region: "us-east-1"
      prefix: "audit-logs/"
      batchSize: 100
      flushInterval: "30s"
```
