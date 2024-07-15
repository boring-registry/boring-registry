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

## Installation

Boring-registry can be installed in various ways, among others we offer a container image and also support the installation with Helm on Kubernetes.
Learn more about the installation [in our documentation](https://boring-registry.github.io/boring-registry/v0.14.0/installation/helm/).

## Configuration

Check out the full documentation at [boring-registry.github.io/boring-registry](https://boring-registry.github.io/boring-registry/latest/configuration/introduction/).
