# boring-registry

Boring-Registry is an open source Terraform Module and Provider registry.

With the boring-registry, you can run a registry to upload and distribute your own modules and providers, as an alternative to publishing them on the public Terraform Registry.

The registry is designed to be simple - there are no external dependencies apart from the storage backend.
It also does not ship with a UI.

It implements the [Module Registry Protocol](https://www.terraform.io/internals/module-registry-protocol) and [Provider Registry Protocol](https://www.terraform.io/internals/provider-registry-protocol) and works out of the box with Terraform version `v0.13.0` and later.

## Installation 

### Helm

```bash
helm upgrade --install --wait --namespace default boring-registry \
oci://ghcr.io/tiermobility/charts/boring-registry
```

### Docker Image

Images are published to [`ghcr.io/tiermobility/boring-registry`](https://github.com/tiermobility/boring-registry/pkgs/container/boring-registry) for every tagged release of the project.

### Local

Run `make` to build the project and install the `boring-registry` executable into `$GOPATH/bin`. Then
start the server with `$GOPATH/bin/boring-registry`, or if `$GOPATH/bin` is already on your `$PATH`,
you can simply run `boring-registry`.

## Overview

The Boring-Registry comes with an `upload` and a `server` command.
* `server` serves both the Module and Provider Registry Protocol for Terraform
* `upload` lets users publish their modules to the registry

## Configuration

The Boring-Registry does not rely on any configuration files. Instead, everything can be configured using flags or environment variables.

**Important Note**:
* Flags have higher priority than environment variables
* All environment variables are prefixed with `BORING_REGISTRY_`

**Example:** To enable debug logging you can either pass the flag: `--debug` or set the environment variable: `BORING_REGISTRY_DEBUG=true`.

### Storage backend

To run the server you need to specify which storage backend to use:

**Minimal configuration using the S3 storage backend:**

```bash
$ boring-registry server \
  --storage-s3-bucket=terraform-registry-test
```

Make sure the server has AWS credentials set (e.g. `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`).

**Minimal example using the GCS storage backend:**

```bash
$ boring-registry server \
  --storage-gcs-bucket=terraform-registry-test
```

Make sure the server has GCP credentials set (e.g. `GOOGLE_CLOUD_PROJECT`).

**Minimal example using the S3 storage backend with MinIO:**

```bash
$ boring-registry server \
  --storage-s3-bucket=terraform-registry-test \
  --storage-s3-region=eu-east-1 \
  --storage-s3-pathstyle=true \
  --storage-s3-endpoint=https://minio.example.com
```

The storage backend has to be specified for the `upload` command as well. Check the [module upload](README.md#modules) section below.

### Authentication

The Boring-Registry can be configured with a set of API keys to match for by using the `--auth-static-token="very-secure-token"` flag or by providing it as an environment variable `BORING_REGISTRY_AUTH_STATIC_TOKEN="very-secure-token"`.

Multiple API keys can be passed by passing the tokens comma-separated to the `--auth-static-token="first-token,second-token"` flag or environment variable `BORING_REGISTRY_AUTH_STATIC_TOKEN="first-token,second-token"`.

The token can be passed to Terraform inside the [`~/.terraformrc` configuration file](https://www.terraform.io/cli/config/config-file#credentials-1):

```hcl
credentials "boring-registry.example.com" {
  token = "very-secure-token"
}
```

## Internal Storage Layout

The Boring-Registry is using the following storage layout inside the storage backend:

```bash
<bucket_prefix>
├── modules
│   └── <namespace>
│       └── <name>
│           └── <provider>
│               ├── <namespace>-<name>-<provider>-<version>.tar.gz
│               └── <namespace>-<name>-<provider>-<version>.tar.gz
└── providers
    └── <namespace>
        ├── signing-keys.json
        └── <name>
            ├── terraform-provider-<name>_<version>_SHA256SUMS
            ├── terraform-provider-<name>_<version>_SHA256SUMS.sig
            └── terraform-provider-<name>_<version>_<os>_<arch>.zip
```

* The `<bucket_prefix>` is an optional prefix under which the Boring-Registry storage is organized and can be set with the `--storage-s3-prefix` or `--storage-gcs-prefix` flags.

An example without any placeholders could be the following.
```bash
<bucket_prefix>
├── modules
│   └── tier
│       └── tls-private-key
│           └── aws
│               ├── tier-tls-private-key-aws-0.1.0.tar.gz
│               └── tier-tls-private-key-aws-0.1.1.tar.gz
└── providers
    └── tier
        ├── signing-keys.json
        └── dummy
            ├── terraform-provider-dummy_0.1.0_SHA256SUMS
            ├── terraform-provider-dummy_0.1.0_SHA256SUMS.sig
            ├── terraform-provider-dummy_0.1.0_linux_amd64.zip
            └── terraform-provider-dummy_0.1.0_linux_arm64.zip
```

## Publishing Modules

Example Terraform configuration using a module referenced from the registry:

```hcl
module "tls-private-key" {
  source = "boring-registry.example.com/hashicorp/tls-private-key/aws"
  version = "~> 0.1"
}
```

### Uploading modules using the CLI

Modules can be published to the registry with the `upload` command.
The command expects a directory as argument, which is then walked recursively in search of `boring-registry.hcl` files.

The `boring-registry.hcl` file should be placed in the root directory of the module and should contain a `metadata` block like the following:

```hcl
metadata {
  namespace = "tier"
  name      = "tls-private-key"
  provider  = "aws"
  version   = "0.1.0"
}
```

When running the upload command, the module is then packaged up and published to the registry.

### Recursive vs. non-recursive upload

Walking the directory recursively is the default behavior of the `upload` command. This way all modules underneath the
current directory will be checked for `boring-registry.hcl` files and modules will be packaged and uploaded if they not
already exist. However, this can be unwanted in certain situations e.g. if a `.terraform` directory is present containing
other modules that have a configuration file. The `--recursive=false` flag will omit this behavior.

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
This would for example be useful to prevent publishing releases from non-`main` branches, while allowing pre-releases to test out pull requests for example.

## Publishing Providers

Example Terraform configuration using a provider referenced from the registry:

```hcl
terraform {
  required_providers {
    dummy = {
      source  = "boring-registry.example.com/tier/dummy"
      version = "0.1.0"
    }
  }
}
```

Providers cannot be uploaded using the CLI yet, so they need to be uploaded outside the Boring-Registry.

The Boring Registry expects a file called `signing-keys.json` to be placed under the `<namespace>` level inside the storage backend.
More information about the purpose of this file can be found in the [Provider Registry Protocol](https://www.terraform.io/internals/provider-registry-protocol#signing_keys).

The file should look like this:

```json
{
  "key_id": "GPG_KEY_ID",
  "ascii_armor": "ASCII_ARMOR"
}
```

For general information on how to build and publish providers for Terraform see the official docs:
https://www.terraform.io/docs/registry/providers.

### Publish providers with Goreleaser
Goreleaser can be used to build providers. Example `.goreleaser.yaml` configuration file:

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
