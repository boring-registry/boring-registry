# boring-registry

Boring Registry is an open source Terraform Module Registry.

The registry is designed to be simple and only implements the "Module Registry Protocol" and apart from the registry storage backend (currently only S3 is supported), there are no external dependencies, it also does not ship with a UI. The endpoints provided are:

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

To run the server you need to specify which registry to use (currently only S3 is supported):

**Example using the S3 registry:**
```bash
$ boring-registry server \
  -type=s3 \
  -s3.bucket=terraform-registry-test
```

To upload modules to the registry you need to specify which registry to use (currently only S3 is supported) and which local directory to work from.

**Example using the S3 registry:**
```bash
$ boring-registry upload \
  -type=s3 \
  -s3.bucket=terraform-registry-test terraform/modules
```

## Configuration

The Boring Registry does not rely on any configuration files. Instead, everything can be configured using flags or environment variables.
**Important Note**: Flags have higher priority than environment variables. Environment variables are always prefixed with `BORING_REGISTRY`.

**Example:**
To enable debug logging you can either pass the flag: `-debug` or set the environment variable: `BORING_REGISTRY_DEBUG=true`.

### Authentication
The Boring Registry can be configured with a set of API keys to match for by using the `-api-key="very-secure-token"` flag or by providing it as an environment variable `BORING_REGISTRY_API_KEY="very-secure-token"`

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

## Help output

```bash
USAGE
  boring-registry [flags] <subcommand> [flags] [<arg>...]

SUBCOMMANDS
  server  run the registry api server
  upload  uploads terraform modules to a registry

FLAGS
  -debug false     log debug output
  -no-color false  disable color output
  -s3-bucket ...   s3 bucket to use for the S3 registry
  -s3-prefix ...   s3 prefix to use for the S3 registry
  -s3-region ...   s3 region to use for the S3 registry
  -type s3         registry type

VERSION
boring-registry v0.1.0
```

### Server help output 

```bash
USAGE
  server [flags]

Run the registry API server.

The server command expects some configuration, such as which registry type to use.
The default registry type is "s3" and is currently the only registry type available.
For more options see the available options below.

EXAMPLE USAGE

boring-registry server -type=s3 -s3-bucket=my-bucket

FLAGS
  -api-key ...                     comma-delimited list of api keys
  -debug false                     log debug output
  -listen-address :5601            listen address for the registry api
  -no-color false                  disable color output
  -s3-bucket ...                   s3 bucket to use for the S3 registry
  -s3-prefix ...                   s3 prefix to use for the S3 registry
  -s3-region ...                   s3 region to use for the S3 registry
  -telemetry-listen-address :7801  listen address for telemetry
  -type s3                         registry type

VERSION
boring-registry v0.1.0
```

### Upload help output

```bash
USAGE
  upload [flags] <dir>

Upload modules to a registry.

The upload command expects some configuration, such as which registry type to use and which local directory to work in.
The default registry type is "s3" and is currently the only registry type available.
For more options see the available options below.

EXAMPLE USAGE

boring-registry upload -type=s3 -s3-bucket=my-bucket terraform/modules


FLAGS
  -debug false     log debug output
  -no-color false  disable color output
  -s3-bucket ...   s3 bucket to use for the S3 registry
  -s3-prefix ...   s3 prefix to use for the S3 registry
  -s3-region ...   s3 region to use for the S3 registry
  -type s3         registry type

VERSION
boring-registry v0.1.0
```

# Roadmap

The project is in its very early stages and there is a lot of things we want to tackle. This may mean some breaking changes in the future, but once the project is stable enough we will put quite heavy focus on keeping changes backwards compatible. This project started out as a single server (just serving the Module Registry Protocol), but is now becoming a single binary that can host the server and allow operators to manage the registry using a streamlined interface.

* Module maintenance - The CLI should be able to inspect/view, modify and delete existing modules.
* Migration helpers - We want the CLI to be able to provide some automation when migrating to the boring-registry. This is currently a manual task and is quite time consuming.
* Extensive metrics - We want to add proper metrics for the server component so we can monitor the internal health of the Boring Registry.
