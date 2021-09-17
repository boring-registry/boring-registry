# boring-registry

Boring Registry is an open source Terraform Module Registry.

The registry is designed to be simple and only implements the "Module Registry Protocol" and apart from the registry storage backend, there are no external dependencies, it also does not ship with a UI. The endpoints provided are:

* GET /v1/modules/{namespace}/{name}/{provider}/versions
* GET /v1/modules/{namespace}/{name}/{provider}/{version}/download

The storage backend expects a clear path structure to know where modules live.
Example structure:
```
namespace=tier/name=s3/provider=aws/version=1.0.0
```

An example bucket looks like this when all modules have been uploaded:

```
$> tree .
.
└── namespace=tier
    ├── name=s3
    │   └── provider=aws
    │       └── version=1.0.0
    │           └── tier-s3-aws-1.0.0.tar.gz
    └── name=dynamodb
        └── provider=aws
            ├── version=1.0.0
            │   └── tier-dynamodb-aws-1.0.0.tar.gz
            └── version=1.0.1
                └── tier-dynamodb-aws-1.0.1.tar.gz

```

Example Terraform configuration file referencing the registry:
```hcl
module "main-s3" {
  source = "boring-registry/tier/s3/aws"
  version = "~> 1"
}
```

## Getting Started

The registry supports two modes:
  * Server - The server runs the registry API
  * Upload - Uploads modules to the configured registry

To run the server you need to specify which registry to use:

**Example using the S3 registry:**
```bash
$ boring-registry server \
  --storage-s3-bucket=terraform-registry-test
```

**Example using the registry with GCS:**
```bash
$ boring-registry server \
  --storage-gcs-bucket=terraform-registry-test
```
Make sure the server has GCP credentials context set properly (e.g. `GOOGLE_CLOUD_PROJECT`). 

**Example using the S3 registry with MINIO:**
```bash
$ boring-registry server \
  --storage-s3-bucket=terraform-registry-test \
  --storage-s3-pathstyle=true \
  --storage-s3-endpoint=https://minio.example.com
```
To upload modules to the registry you need to specify which registry to use (currently only S3 is supported) and which local directory to work from.

## Configuration

The Boring Registry does not rely on any configuration files. Instead, everything can be configured using flags or environment variables.
**Important Note**: Flags have higher priority than environment variables. Environment variables are always prefixed with `BORING_REGISTRY`.

**Example:**
To enable debug logging you can either pass the flag: `--debug` or set the environment variable: `BORING_REGISTRY_DEBUG=true`.

### Authentication
The Boring Registry can be configured with a set of API keys to match for by using the `--api-key="very-secure-token"` flag or by providing it as an environment variable `BORING_REGISTRY_API_KEY="very-secure-token"`

This can then be configured inside `~/.terraformrc` like this:
```
credentials "boring-registry" {
  token = “very-secure-token”
}
```

## Uploading modules

When uploading modules the `upload` command expects a directory. This directory is then walked recursively and looks for files called: `boring-registry.hcl`.

The `boring-registry.hcl` file expects a `metadata` block like this:
```hcl
metadata {
  namespace = "tier"
  name      = "s3"
  provider  = "aws"
  version   = "1.0.0"
}
```

When running the upload command, the module is then packaged up and stored inside the registry. 

### Recursive vs. non-recursive upload

Walking the directory recursively is the default behavior of `boring-registry upload`. This way all modules underneath the
current directory will be checked for `boring-registry.hcl` files and modules will be packaged and uploaded if they not
already exist. However this can be unwanted in certain situations e.g. if a `.terraform` directory is present containing
other modules that have a configuration file. The `--recursive=false` flag will omit this behavior. Here is a short example:

### Fail early if module version already exists

By default the upload command will silently ignore already uploaded versions of a module and return exit code `0`. For
tagging mono-repositories this can become a problem as it is not clear if the module version is new or already uploaded.
The `--ignore-existing=false` parameter will force the upload command to return exit code `1` in such a case. In
combination with `--recursive=false` the exit code can be used to tag the GIT repository only if a new version was uploaded.

```shell
for i in $(ls -d */); do
  printf "Operating on module \"${i%%/}\"\n"
  # upload the given directory
  ./boring-registry upload --type gcs -gcs-bucket=my-boring-registry-upload-bucket --recursive=false --ignore-existing=false ${i%%/}
  # tag the repo with a tag composed out of the boring-registry.hcl if not already exist
  if [ $? -eq 0 ]; then
    # git tag the repository with the version from boring-registry.hcl
    # hint: use mattolenik/hclq to parse the hcl file
  fi
done
```

### Module version constraints

The `--version-constraints-semver` flag lets you specify a range of acceptable semver versions for modules.
It expects a specially formatted string containing one or more conditions, which are separated by commas.
The syntax is similar to the [Terraform Version Constraint Syntax](https://www.terraform.io/docs/language/expressions/version-constraints.html#version-constraint-syntax).

In order to exclude all SemVer pre-releases, you can e.g. use `--version-constraints-semver=">=v0"`, which will instruct the boring-registry cli to only upload non-pre-releases to the registry.
This would for example be useful to restrict CI to only publish releases from the `main` branch.

The `--version-constraints-regex` flag lets you specify a regex that module versions have to match.
In order to only match pre-releases, you can e.g. use `--version-constraints-regex="^[0-9]+\.[0-9]+\.[0-9]+-|\d*[a-zA-Z-][0-9a-zA-Z-]*$"`.
This would for example be useful to prevent publishing releases from non-`main` branches, while allowing pre-releases to test out e.g. pull-requests.

## Help output
```
Usage:
  boring-registry [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  help        Help about any command
  server      Starts the server component
  upload      Upload modules
  version     Prints the version of the Boring Registry

Flags:
      --debug                                        Enable debug logging
  -h, --help                                         help for boring-registry
      --json                                         Enable json logging
      --storage-gcs-bucket string                    Bucket to use when using the GCS registry type
      --storage-gcs-prefix string                    Prefix to use when using the GCS registry type
      --storage-gcs-sa-email string                  Google service account email to be used for Application Default Credentials (ADC)
                                                     GOOGLE_APPLICATION_CREDENTIALS environment variable might be used as alternative.
                                                     For GCS presigned URLs this SA needs the iam.serviceAccountTokenCreator role.
      --storage-gcs-signedurl                        Generate GCS signedURL (public) instead of relying on GCP credentials being set on terraform init.
                                                     WARNING: only use in combination with api-key option.
      --storage-gcs-signedurl-expiry gcs-signedurl   Generate GCS signed URL valid for X seconds. Only meaningful if used in combination with gcs-signedurl (default 30s)
      --storage-s3-bucket string                     S3 bucket to use for the registry
      --storage-s3-endpoint string                   S3 bucket endpoint URL (required for MINIO)
      --storage-s3-pathstyle                         S3 use PathStyle (required for MINIO)
      --storage-s3-prefix string                     S3 bucket prefix to use for the registry
      --storage-s3-region string                     S3 bucket region to use for the registry

Use "boring-registry [command] --help" for more information about a command.
```

### Server help output 

```
Starts the server component

Usage:
  boring-registry server [flags]

Flags:
      --api-key string                    Comma-separated string of static API keys to protect the server with
  -h, --help                              help for server
      --listen-address string             Address to listen on (default ":5601")
      --listen-telemetry-address string   Telemetry address to listen on (default ":7801")
      --tls-cert-file string              TLS certificate to serve
      --tls-key-file string               TLS private key to serve

Global Flags:
      --debug                                        Enable debug logging
      --json                                         Enable json logging
      --storage-gcs-bucket string                    Bucket to use when using the GCS registry type
      --storage-gcs-prefix string                    Prefix to use when using the GCS registry type
      --storage-gcs-sa-email string                  Google service account email to be used for Application Default Credentials (ADC)
                                                     GOOGLE_APPLICATION_CREDENTIALS environment variable might be used as alternative.
                                                     For GCS presigned URLs this SA needs the iam.serviceAccountTokenCreator role.
      --storage-gcs-signedurl                        Generate GCS signedURL (public) instead of relying on GCP credentials being set on terraform init.
                                                     WARNING: only use in combination with api-key option.
      --storage-gcs-signedurl-expiry gcs-signedurl   Generate GCS signed URL valid for X seconds. Only meaningful if used in combination with gcs-signedurl (default 30s)
      --storage-s3-bucket string                     S3 bucket to use for the registry
      --storage-s3-endpoint string                   S3 bucket endpoint URL (required for MINIO)
      --storage-s3-pathstyle                         S3 use PathStyle (required for MINIO)
      --storage-s3-prefix string                     S3 bucket prefix to use for the registry
      --storage-s3-region string                     S3 bucket region to use for the registry
```

### Upload help output
```
Upload modules

Usage:
  boring-registry upload [flags] MODULE

Flags:
  -h, --help                                help for upload
      --ignore-existing                     Ignore already existing modules. If set to false upload will fail immediately if a module already exists in that version (default true)
      --recursive                           Recursively traverse <dir> and upload all modules in subdirectories (default true)
      --version-constraints-regex string    Limit the module versions that are eligible for upload with a regex that a version has to match.
                                            Can be combined with the -version-constraints-semver flag
      --version-constraints-semver string   Limit the module versions that are eligible for upload with version constraints.
                                            The version string has to be formatted as a string literal containing one or more conditions, which are separated by commas. Can be combined with the -version-constrained-regex flag

Global Flags:
      --debug                                        Enable debug logging
      --json                                         Enable json logging
      --storage-gcs-bucket string                    Bucket to use when using the GCS registry type
      --storage-gcs-prefix string                    Prefix to use when using the GCS registry type
      --storage-gcs-sa-email string                  Google service account email to be used for Application Default Credentials (ADC)
                                                     GOOGLE_APPLICATION_CREDENTIALS environment variable might be used as alternative.
                                                     For GCS presigned URLs this SA needs the iam.serviceAccountTokenCreator role.
      --storage-gcs-signedurl                        Generate GCS signedURL (public) instead of relying on GCP credentials being set on terraform init.
                                                     WARNING: only use in combination with api-key option.
      --storage-gcs-signedurl-expiry gcs-signedurl   Generate GCS signed URL valid for X seconds. Only meaningful if used in combination with gcs-signedurl (default 30s)
      --storage-s3-bucket string                     S3 bucket to use for the registry
      --storage-s3-endpoint string                   S3 bucket endpoint URL (required for MINIO)
      --storage-s3-pathstyle                         S3 use PathStyle (required for MINIO)
      --storage-s3-prefix string                     S3 bucket prefix to use for the registry
      --storage-s3-region string                     S3 bucket region to use for the registry
```

# Roadmap

The project is in its very early stages and there is a lot of things we want to tackle. This may mean some breaking changes in the future, but once the project is stable enough we will put quite heavy focus on keeping changes backwards compatible. This project started out as a single server (just serving the Module Registry Protocol), but is now becoming a single binary that can host the server and allow operators to manage the registry using a streamlined interface.

* Module maintenance - The CLI should be able to inspect/view, modify and delete existing modules.
* Migration helpers - We want the CLI to be able to provide some automation when migrating to the boring-registry. This is currently a manual task and is quite time consuming.
* Extensive metrics - We want to add proper metrics for the server component so we can monitor the internal health of the Boring Registry.
