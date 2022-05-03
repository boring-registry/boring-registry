# boring-registry

Boring Registry is an open source Terraform Provider and Module Registry.

The registry is designed to be simple and implements the "Provider Registry Protocol" and "Module Registry Protocol" and apart from the storage backend, there are no external dependencies, it also does not ship with a UI. 

## Module Registry Protocol

The Boring Registry expects a defined path structure inside the storage backend.

**Please note**: Modules must be stored under `${storage}/${prefix}/modules/`

Example tree:

```shell
bucket/modules
└── namespace=tier
    └── name=test
        └── provider=dummy
            └── version=1.0.0
                └── tier-test-dummy-1.0.0.tar.gz
```

Example Terraform configuration referencing the registry:

```hcl
module "test" {
  source = "boring-registry/tier/test/dummy"
  version = "~> 1"
}
```

### Endpoints 

The endpoints provided by the Module Registry Protocol are:

* `GET /v1/modules/:namespace/:name/:provider/versions`
* `GET /v1/modules/:namespace/:name/:provider/:version/download`



## Provider Registry Protocol

Similar to the Module Registry Protocol, the Boring Registry expects a defined path structure inside the storage backend.

**Please note**: Providers must be stored under `${storage}/${prefix}/providers/`

```shell
bucket/providers
└── namespace=tier
    ├── name=dummy
    │   └── version=1.0.0
    │       ├── os=linux
    │       │   └── arch=amd64
    │       │       └── terraform-provider-dummy_1.0.0_linux_amd64.zip
    │       ├── terraform-provider-dummy_1.0.0_SHA256SUMS
    │       └── terraform-provider-dummy_1.0.0_SHA256SUMS.sig
    └── signing-keys.json
```

Example Terraform configuration referencing the registry:

```hcl
terraform {
  required_providers {
    dummy = {
      source  = "boring-registry/tier/dummy"
      version = "1.0.0"
    }
  }
}
```

### Endpoints 

The endpoints provided by the Provider Registry Protocol are:

* `GET /v1/providers/:namespace/:name/versions`
* `GET /v1/providers/:namespace/:name/:version/download/:os/:arch`

# Getting Started

The Boring Registry comes with a server component that serves both the Module and Provider Registry Protocol but also comes with an upload subcommand that can upload modules to a storage backend:

  * Server - Runs the server component
  * Upload - Uploads modules to the configured storage backend

To run the server you need to specify which storage backend to use:

**Example using the S3 storage:**

```bash
$ boring-registry server \
  --storage-s3-bucket=terraform-registry-test
```

**Example using the GCS storage:**

```bash
$ boring-registry server \
  --storage-gcs-bucket=terraform-registry-test
```

Make sure the server has GCP credentials context set properly (e.g. `GOOGLE_CLOUD_PROJECT`). 

**Example using the S3 storage with MINIO:**

```bash
$ boring-registry server \
  --storage-s3-bucket=terraform-registry-test \
  --storage-s3-pathstyle=true \
  --storage-s3-endpoint=https://minio.example.com
```

To upload modules to the storage backend you need to specify which storage to use and which local directory to use.

## Configuration

The Boring Registry does not rely on any configuration files. Instead, everything can be configured using flags or environment variables.

**Important Note**: Flags have higher priority than environment variables. 
Environment variables are always prefixed with `BORING_REGISTRY`.

**Example:**
To enable debug logging you can either pass the flag: `--debug` or set the environment variable: `BORING_REGISTRY_DEBUG=true`.

To enable json log output you can either pass the flag: `--json` or set the environment variable: `BORING_REGISTRY_JSON=true`.

To specify the s3 bucket you can either pass the flag: `--storage-s3-bucket=${bucket}` or set the environment variable: `BORING_REGISTRY_STORAGE_S3_BUCKET=${bucket}`

### Authentication

The Boring Registry can be configured with a set of API keys to match for by using the `--api-key="very-secure-token"` flag or by providing it as an environment variable `BORING_REGISTRY_API_KEY="very-secure-token"`

This can then be configured inside `~/.terraformrc` like this:

```hcl
credentials "boring-registry" {
  token = “very-secure-token”
}
```

# Modules

Modules can either be uploaded directly to the storage backend or by using the subcommand `upload`.

## Uploading modules using the CLI

When uploading modules the `upload` command expects a directory. This directory is then walked recursively and looks for files called: `boring-registry.hcl`.

The `boring-registry.hcl` file expects a `metadata` block like this:

```hcl
metadata {
  namespace = "tier"
  name      = "test"
  provider  = "dummy"
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



# Providers

Providers cannot be uploaded using the CLI yet so they need to be uploaded outside of the Boring Registry.

The Boring Registry expects a file called `signing-keys.json` to be placed under the `namespace` level inside the storage backend.

This file should look like this:

```json
{
  "key_id": "GPG_KEY_ID",
  "ascii_armor": "ASCII_ARMOR"
}
```

Goreleaser can be used to build providers. Example .goreleaser.yaml configuration file:

```yaml
# Visit https://goreleaser.com for documentation on how to customize this
# behavior.
before:
  hooks:
    # this is just an example and not a requirement for provider building/publishing
    - go mod tidy
builds:
  - env:
      # goreleaser does not work with CGO, it could also complicate
      # usage by users in CI/CD systems like Terraform Cloud where
      # they are unable to install libraries.
      - CGO_ENABLED=0
    mod_timestamp: "{{ .CommitTimestamp }}"
    flags:
      - -trimpath
    ldflags:
      - "-s -w -X main.version={{.Version}} -X main.commit={{.Commit}}"
    goos:
      - freebsd
      - windows
      - linux
      - darwin
    goarch:
      - amd64
      - "386"
      - arm
      - arm64
    ignore:
      - goos: darwin
        goarch: "386"
    binary: "{{ .ProjectName }}_v{{ .Version }}"
archives:
  - format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_SHA256SUMS"
  algorithm: sha256
signs:
  - artifacts: checksum
release:
  # If you want to manually examine the release before its live, uncomment this line:
  # draft: true
changelog:
  skip: true
```

For general information on how to build and publish providers for Terraform see the official docs:
https://www.terraform.io/docs/registry/providers.

# Installation

## Docker Image

Images are published to [`ghcr.io/tiermobility/boring-registry`](https://github.com/tiermobility/boring-registry/pkgs/container/boring-registry) for every tagged release of the project.

## Local

Run `make` to build the project and install the `boring-registry` executable into `$GOPATH/bin`. Then
start the server with `$GOPATH/bin/boring-registry`, or if `$GOPATH/bin` is already on your `$PATH`,
you can simply run `boring-registry`.

# Roadmap

The project is in its very early stages and there is a lot of things we want to tackle. This may mean some breaking changes in the future, but once the project is stable enough we will put quite heavy focus on keeping changes backwards compatible. This project started out as a single server (just serving the Module Registry Protocol), but is now becoming a single binary that can host the server and allow operators to manage the registry using a streamlined interface.

* Module maintenance - The CLI should be able to inspect/view, modify and delete existing modules.
* Migration helpers - We want the CLI to be able to provide some automation when migrating to the boring-registry. This is currently a manual task and is quite time consuming.
* Extensive metrics - We want to add proper metrics for the server component so we can monitor the internal health of the Boring Registry.
