# OCI Registry

## Authorization

The boring-registry needs to authenticate with your OCI-compatible container registry. This can be achieved through:

1. **Environment Variables**: Set the `OCI_REGISTRY_USERNAME` and `OCI_REGISTRY_PASSWORD` environment variables for basic authentication
2. **Registry Token**: Use `OCI_REGISTRY_TOKEN` for token-based authentication
3. **Docker Config**: The registry client will also check standard Docker configuration files (`~/.docker/config.json`)

For public registries like Docker Hub, you might not need authentication for pulling artifacts.

## Configuration for OCI Registry

The following configuration options are available:

|Flag|Environment Variable|Description|
|---|---|---|
|`--storage-oci-registry`|`BORING_REGISTRY_STORAGE_OCI_REGISTRY`|OCI registry hostname (e.g., registry.example.com)|
|`--storage-oci-repository`|`BORING_REGISTRY_STORAGE_OCI_REPOSITORY`|Base repository path in the registry (e.g., boring-registry)|
|`--storage-oci-username`|`BORING_REGISTRY_STORAGE_OCI_USERNAME` or `OCI_REGISTRY_USERNAME`|Username for registry authentication (optional)|
|`--storage-oci-password`|`BORING_REGISTRY_STORAGE_OCI_PASSWORD` or `OCI_REGISTRY_PASSWORD`|Password for registry authentication (optional)|
|`--storage-oci-token`|`BORING_REGISTRY_STORAGE_OCI_TOKEN` or `OCI_REGISTRY_TOKEN`|Token for registry authentication (optional)|
|`--storage-oci-insecure`|`BORING_REGISTRY_STORAGE_OCI_INSECURE`|Allow insecure connections to the registry (default false)|

## OCI Registry Layout

The OCI storage backend organizes Terraform modules and providers using a structured repository layout:

### Modules
- **Path**: `{registry}/{repository}/modules/{namespace}/{name}/{provider}:{namespace}-{name}-{provider}-{version}`
- **Example**: `registry.example.com/boring-registry/modules/hashicorp/consul/aws:hashicorp-consul-aws-1.0.0`

### Providers
- **Provider Package**: `{registry}/{repository}/providers/{namespace}/{name}:terraform-provider-{name}_{version}_{os}_{arch}.zip`
- **SHA256SUMS**: `{registry}/{repository}/providers/{namespace}/{name}:terraform-provider-{name}_{version}_SHA256SUMS`
- **SHA256SUMS Signature**: `{registry}/{repository}/providers/{namespace}/{name}:terraform-provider-{name}_{version}_SHA256SUMS.sig`
- **Signing Keys**: `{registry}/{repository}/providers/{namespace}:signing-keys.json`

### Mirrored Providers
- **Provider Package**: `{registry}/{repository}/mirror/providers/{hostname}/{namespace}/{name}:terraform-provider-{name}_{version}_{os}_{arch}.zip`
- **SHA256SUMS**: `{registry}/{repository}/mirror/providers/{hostname}/{namespace}/{name}:terraform-provider-{name}_{version}_SHA256SUMS`
- **SHA256SUMS Signature**: `{registry}/{repository}/mirror/providers/{hostname}/{namespace}/{name}:terraform-provider-{name}_{version}_SHA256SUMS.sig`
- **Signing Keys**: `{registry}/{repository}/mirror/providers/{hostname}/{namespace}:signing-keys.json`

## Examples

The following shows a minimal example to run `boring-registry server` with an OCI registry:

```console
$ boring-registry server \
  --storage-oci-registry=registry.example.com \
  --storage-oci-repository=boring-registry
```

### Using Docker Hub

```console
$ boring-registry server \
  --storage-oci-registry=docker.io \
  --storage-oci-repository=myorg/boring-registry \
  --storage-oci-username=myusername \
  --storage-oci-password=mypassword
```

### Using a Private Registry with Token Authentication

```console
$ boring-registry server \
  --storage-oci-registry=private-registry.company.com \
  --storage-oci-repository=terraform/boring-registry \
  --storage-oci-token=myregistrytoken
```

### Using Harbor Registry

```console
$ boring-registry server \
  --storage-oci-registry=harbor.company.com \
  --storage-oci-repository=terraform/boring-registry \
  --storage-oci-username=admin \
  --storage-oci-password=Harbor12345
```

## Supported OCI Registries

The OCI storage backend is compatible with any OCI-compliant container registry, including:

- **Docker Hub**
- **Harbor**
- **Azure Container Registry (ACR)**
- **Amazon Elastic Container Registry (ECR)**
- **Google Container Registry (GCR)**
- **GitHub Container Registry (GHCR)**
- **GitLab Container Registry**
- **JFrog Artifactory**
- **Sonatype Nexus Repository**

## Security Considerations

- Always use HTTPS in production environments (avoid `--storage-oci-insecure`)
- Use token-based authentication when available, as it's generally more secure than username/password
- Consider using dedicated service accounts with minimal required permissions
- Regularly rotate authentication credentials
- Monitor registry access logs for unauthorized activity

## Troubleshooting

### Authentication Issues
- Verify that your credentials are correct
- Check that the user has push/pull permissions for the repository
- Ensure the registry supports the authentication method you're using

### Network Issues
- Verify that the registry hostname is reachable
- Check firewall rules if using a private registry
- Use `--storage-oci-insecure` only for testing with self-signed certificates

### Repository Access
- Ensure the repository exists and is accessible
- Check that you have the necessary permissions to create repositories if they don't exist
- Verify the repository naming follows OCI standards
