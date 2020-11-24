# boring-registry

Boring Registry is an open source Terraform Module Registry.

The registry is designed to be simple and only implements the "Module Registry Protocol" and apart from the registry storage backend (currently only S3 is supported), there are no external dependencies, it also does not ship with a UI.

## Getting Started

The registry supports two modes:
  * Server - The server runs the registry API
  * Upload - Uploads modules to the configured registry

To run the server you need to specify which registry to use (currently only S3 is supported):

**Example:**
```bash
boring-registry server -type=s3 -s3.bucket=terraform-registry-test
```

To upload modules to the registry you need to specify which registry to use (currently only S3 is supported) and which local directory to work from.

**Example:**
```bash
boring-registry upload -type=s3 -s3.bucket=terraform-registry-test <DIR> 
```
