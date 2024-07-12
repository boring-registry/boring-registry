# Provider Network Mirror

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

## Pull-through mirror

As part of the Provider Network Mirror, a pull-through mirror can optionally be activated with `--network-mirror-pull-through=true`.

The pull-through functionality makes it possible that the providers do not have to be uploaded upfront to the storage backend.
Instead, boring-registry serves the providers of the origin registry and mirrors them automatically to the storage backend on the first download.
On the subsequent download request, boring-registry serves the providers directly from the storage backend.
This can significantly speed up the `terraform init` phase and in some cases save additional traffic costs.
