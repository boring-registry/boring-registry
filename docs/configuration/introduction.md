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
