# Publish Providers


For general information on how to build and publish providers for Terraform see the [official documentation](https://developer.hashicorp.com/terraform/registry/providers).

## GPG Public Keys

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

## Publishing providers with the CLI

1. Manually prepare the provider release artifacts according to the [documentation from hashicorp](https://developer.hashicorp.com/terraform/registry/providers/publishing#preparing-your-provider)
2. Publish the artifacts with the following (minimal) command:
    ```bash
    boring-registry upload provider \
    --storage-s3-bucket <bucket_name> \
    --namespace <namespace> \
    --filename-sha256sums /absolute/path/to/terraform-provider-<name>_<version>_SHA256SUMS
    ```

## Referencing providers in Terraform

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
