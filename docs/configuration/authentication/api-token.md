# API Token

The boring-registry can be configured with a set of API tokens to match for by using the `--auth-static-token="very-secure-token"` flag or by providing it as an environment variable `BORING_REGISTRY_AUTH_STATIC_TOKEN="very-secure-token"`.

Multiple API tokens can be configured by passing comma-separated tokens to the `--auth-static-token="first-token,second-token"` flag or environment variable `BORING_REGISTRY_AUTH_STATIC_TOKEN="first-token,second-token"`.

## OpenTofu

The token can be passed to OpenTofu inside the [configuration file](https://developer.hashicorp.com/terraform/cli/config/config-file#credentials-1):

```hcl
credentials "boring-registry.example.com" {
  token = "very-secure-token"
}
```

## Terraform

The token can be passed to Terraform inside the [`~/.terraformrc` configuration file](https://opentofu.org/docs/cli/config/config-file/#locations):

```hcl
credentials "boring-registry.example.com" {
  token = "very-secure-token"
}
```

