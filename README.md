# boring-registry

Boring-registry is an open source module and provider registry compatible with Terraform and [OpenTofu](https://github.com/opentofu/opentofu).

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [Overview](#overview)
- [Installation](#installation)
- [Configuration](#configuration)
- [Internal Storage Layout](#internal-storage-layout)
- [Publishing Modules](#publishing-modules)
- [Publishing Providers](#publishing-providers)
- [Provider Network Mirror](#provider-network-mirror)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Overview

With boring-registry, you can upload and distribute your own modules and providers, as an alternative to publishing them on HashiCorp's public Terraform Registry.

Support for the [Module Registry Protocol](https://www.terraform.io/internals/module-registry-protocol), [Provider Registry Protocol](https://www.terraform.io/internals/provider-registry-protocol), and [Provider Network Mirror Protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol) allows it to work natively with Terraform and OpenTofu.

### Features

* Module Registry
* Provider Registry
* Network mirror for providers
* Pull-through mirror for providers
* Support for S3, GCS, Azure Blob Storage, and MinIO object storage

## Installation

### Helm

```bash
helm upgrade --install --wait --namespace default boring-registry oci://ghcr.io/boring-registry/charts/boring-registry
```

### Docker Image

Images are published to [`ghcr.io/boring-registry/boring-registry`](https://github.com/boring-registry/boring-registry/pkgs/container/boring-registry) for every tagged release of the project.

### Local

Run `make` to build the project and install the `boring-registry` executable into `$GOPATH/bin`.
Then start the server with `$GOPATH/bin/boring-registry`, or if `$GOPATH/bin` is already on your `$PATH`, you can simply run `boring-registry`.

## Configuration

The boring-registry does not rely on a configuration file.
Instead, everything can be configured using flags or environment variables.

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

**Minimal example using the Azure storage backend:**

```bash
$ boring-registry server \
  --storage-azure-account=registry \
  --storage-azure-container=terraform-registry-test
```

Make sure the server has Azure credentials set. The Azure backend supports the following authentication methods:

- Environment Variables
  - Service principal with client secret (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`)
  - Service principal with certificate (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_CERTIFICATE_PATH`, `AZURE_CLIENT_CERTIFICATE_PASSWORD`)
  - User with username and password (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_USERNAME`, `AZURE_PASSWORD`)
- Managed Identity
- Azure CLI

Make sure the used identity has the role `Storage Blob Data Contributor` on the Storage Account.

The storage backend has to be specified for the `upload` command as well. Check the [module upload](README.md#modules) section below.

### Authentication

The boring-registry can be configured with a set of API keys to match for by using the `--auth-static-token="very-secure-token"` flag or by providing it as an environment variable `BORING_REGISTRY_AUTH_STATIC_TOKEN="very-secure-token"`.

Multiple API keys can be configured by passing comma-separated tokens to the `--auth-static-token="first-token,second-token"` flag or environment variable `BORING_REGISTRY_AUTH_STATIC_TOKEN="first-token,second-token"`.

The token can be passed to Terraform inside the [`~/.terraformrc` configuration file](https://developer.hashicorp.com/terraform/cli/config/config-file#credentials-1):

```hcl
credentials "boring-registry.example.com" {
  token = "very-secure-token"
}
```

### Proxy

By default the boring-registry return pre-signed URLs, pointing to the remote storage, as download URLs for the Terraform CLI. The boring-registry can be configured with a flag to serve as a proxy to deliver the files directly from boring-registry instead of redirecting to the remote storage.

You can activate the proxy by using the `--proxy` flag or by providing it as an environment variable `BORING_REGISTRY_PROXY=true`.

***Note :** If activated, the flag proxy will be applied to modules and providers, but not mirrors.*

## Internal Storage Layout

The boring-registry is using the following storage layout inside the storage backend:

```bash
<bucket_prefix>
├── modules
│   └── <namespace>
│       └── <name>
│           └── <provider>
│               ├── <namespace>-<name>-<provider>-<version>.tar.gz
│               └── <namespace>-<name>-<provider>-<version>.tar.gz
├── providers
│   └── <namespace>
│       ├── signing-keys.json
│       └── <name>
│           ├── terraform-provider-<name>_<version>_SHA256SUMS
│           ├── terraform-provider-<name>_<version>_SHA256SUMS.sig
│           └── terraform-provider-<name>_<version>_<os>_<arch>.zip
└── mirror
    └── providers
        └── <hostname>
            └── <namespace>
                ├── signing-keys.json
                └── <name>
                    ├── terraform-provider-<name>_<version>_SHA256SUMS
                    ├── terraform-provider-<name>_<version>_SHA256SUMS.sig
                    └── terraform-provider-<name>_<version>_<os>_<arch>.zip
```

The `<bucket_prefix>` is an optional prefix under which the boring-registry storage is organized and can be set with the `--storage-s3-prefix` or `--storage-gcs-prefix` flags.

An example without any placeholders could be the following:

```bash
<bucket_prefix>
├── modules
│   └── acme
│       └── tls-private-key
│           └── aws
│               ├── acme-tls-private-key-aws-0.1.0.tar.gz
│               └── acme-tls-private-key-aws-0.2.0.tar.gz
├── providers
│   └── acme
│       ├── signing-keys.json
│       └── dummy
│           ├── terraform-provider-dummy_0.1.0_SHA256SUMS
│           ├── terraform-provider-dummy_0.1.0_SHA256SUMS.sig
│           ├── terraform-provider-dummy_0.1.0_linux_amd64.zip
│           └── terraform-provider-dummy_0.1.0_linux_arm64.zip
└── mirror
    └── providers
        └── terraform.example.com
            └── acme
                ├── signing-keys.json
                └── random
                    ├── terraform-provider-random_0.1.0_SHA256SUMS
                    ├── terraform-provider-random_0.1.0_SHA256SUMS.sig
                    └── terraform-provider-random_0.1.0_linux_amd64.zip
```

## Publishing Modules

Example Terraform configuration using a module referenced from the registry:

```hcl
module "tls-private-key" {
  source = "boring-registry.example.com/acme/tls-private-key/aws"
  version = "~> 0.1"
}
```

### Uploading modules using the CLI

Modules can be published to the registry with the `upload` command.
The command expects a directory as argument, which is then walked recursively in search of `boring-registry.hcl` files.

The `boring-registry.hcl` file should be placed in the root directory of the module and should contain a `metadata` block like the following:

```hcl
metadata {
  namespace = "acme"
  name      = "tls-private-key"
  provider  = "aws"
  version   = "0.1.0"
}
```

When running the upload command, the module is then packaged up and published to the registry.

### Recursive vs. non-recursive upload

Walking the directory recursively is the default behavior of the `upload` command.
This way all modules underneath the current directory will be checked for `boring-registry.hcl` files and modules will be packaged and uploaded if they not already exist
However, this can be unwanted in certain situations e.g. if a `.terraform` directory is present containing other modules that have a configuration file.
The `--recursive=false` flag will omit this behavior.

### Fail early if module version already exists

By default the upload command will silently ignore already uploaded versions of a module and return exit code `0`.
For tagging mono-repositories this can become a problem as it is not clear if the module version is new or already uploaded.
The `--ignore-existing=false` parameter will force the upload command to return exit code `1` in such a case.
In combination with `--recursive=false` the exit code can be used to tag the Git repository only if a new version was uploaded.

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

For general information on how to build and publish providers for Terraform see the [official documentation](https://developer.hashicorp.com/terraform/registry/providers).

### GPG Public Keys

The boring-registry expects a file named `signing-keys.json` to be placed under the `<namespace>` level in the storage backend.
More information about the purpose of this file can be found in the [Provider Registry Protocol](https://developer.hashicorp.com/terraform/internals/provider-registry-protocol#signing_keys).

The file should have the following format:

```json
{
  "gpg_public_keys": [
    {
      "key_id": "51852D87348FFC4C",
      "ascii_armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n..."
    }
  ]
}
```

Multiple public keys are supported by extending the `gpg_public_keys` array.

The `v0.10.0` and previous releases of the boring-registry only supported a single signing key in the following format:

```json
{
  "key_id": "51852D87348FFC4C",
  "ascii_armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n..."
}
```

### Publishing providers with the CLI

1. Manually prepare the provider release artifacts according to the [documentation from hashicorp](https://developer.hashicorp.com/terraform/registry/providers/publishing#preparing-your-provider)
2. Publish the artifacts with the following (minimal) command:
    ```bash
    boring-registry upload provider \
    --storage-s3-bucket <bucket_name> \
    --namespace <namespace> \
    --filename-sha256sums /absolute/path/to/terraform-provider-<name>_<version>_SHA256SUMS
    ```

### Referencing providers in Terraform

Example Terraform configuration using a provider referenced from the registry:

```hcl
terraform {
  required_providers {
    dummy = {
      source  = "boring-registry.example.com/acme/dummy"
      version = "0.1.0"
    }
  }
}
```

## Provider Network Mirror

> [!NOTE]
> The Provider Network Mirror feature is available starting from `v0.12.0`.
> The Network Mirror is enabled by default, but can be disabled with `--network-mirror=false`.

The boring-registry implements the [Provider Network Mirror Protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol) to provide an alternative installation source for providers.

Check the [Terraform CLI documentation](https://developer.hashicorp.com/terraform/cli/config/config-file#provider-installation) to learn how to configure Terraform to use the provider network mirror.
In the following is an example for a `.terraformrc`:
```hcl
provider_installation {
  network_mirror {
    url = "https://boring-registry.example.com:5601/v1/mirror/"
  }
}
```

To populate the mirror, the provider release artifacts need to be uploaded to the storage backend.
Refer to the [Internal Storage Layout](#internal-storage-layout) section for an overview of the required structure.
The [`terraform providers mirror`](https://developer.hashicorp.com/terraform/cli/commands/providers/mirror) command is a good starting point for collecting the necessary files.

### Pull-through mirror

As part of the Provider Network Mirror, a pull-through mirror can optionally be activated with `--network-mirror-pull-through=true`.

The pull-through functionality makes it possible that the providers do not have to be uploaded upfront to the storage backend.
Instead, boring-registry serves the providers of the origin registry and mirrors them automatically to the storage backend on the first download.
On the subsequent download request, boring-registry serves the providers directly from the storage backend.
This can significantly speed up the `terraform init` phase and in some cases save additional traffic costs.
