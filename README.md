# boring-registry

Boring-registry is a simple open source module and provider registry compatible with Terraform and [OpenTofu](https://github.com/opentofu/opentofu).

## Overview

With boring-registry, you can upload and distribute your own modules and providers, as an alternative to publishing them on HashiCorp's public Terraform Registry.

Support for the [Module Registry Protocol](https://www.terraform.io/internals/module-registry-protocol), [Provider Registry Protocol](https://www.terraform.io/internals/provider-registry-protocol), and [Provider Network Mirror Protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol) allows it to work natively with Terraform and OpenTofu.

### Features

* Module Registry
* Provider Registry
* Network mirror for providers
* Pull-through mirror for providers
* Support for S3, GCS, Azure Blob Storage, and MinIO object storage
* Support for OIDC and static API token authorization

## Installation

Boring-registry can be installed in various ways, among others we offer a container image and also support the installation with Helm on Kubernetes.
Learn more about the installation [in our documentation](https://boring-registry.github.io/boring-registry/latest/installation/helm/).

## Configuration

Check out the full documentation at [boring-registry.github.io/boring-registry](https://boring-registry.github.io/boring-registry/latest/configuration/introduction/).

## Contributing

### Setup

Tools:
- `go`
- [pre-commit](https://pre-commit.com/)
- [golangci-lint](https://golangci-lint.run/)
- `mkdocs`

#### pre-commit

Install pre-commit hooks:

```shell
pre-commit install --install-hooks
```

### docs

The docs can be rendered and served with `mkdocs serve` to preview changes live in the browser.

```shell
docker build -f docs/Dockerfile -t boring-registry-docs .
docker run --rm -it -p 8000:8000 -v ${PWD}:/docs boring-registry-docs
```
