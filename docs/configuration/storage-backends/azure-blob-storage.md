# Azure Blob Storage

## Authorization

Make sure the server has Azure credentials set.
The Azure backend supports the following authentication methods:

- Environment Variables
  - Service principal with client secret (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`)
  - Service principal with certificate (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_CERTIFICATE_PATH`, `AZURE_CLIENT_CERTIFICATE_PASSWORD`)
  - User with username and password (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_USERNAME`, `AZURE_PASSWORD`)
- Managed Identity
- Azure CLI

Make sure the used identity has the role `Storage Blob Data Contributor` on the Storage Account.

## Configuration for Azure Blob Storage

The following configuration options are available:

|Flag|Environment Variable|Description|
|---|---|---|
|`--storage-azure-account`|`BORING_REGISTRY_STORAGE_AZURE_ACCOUNT`|Azure Storage Account to use for the registry|
|`--storage-azure-container`|`BORING_REGISTRY_STORAGE_AZURE_CONTAINER`|Azure Storage Container to use for the registry|
|`--storage-azure-prefix`|`BORING_REGISTRY_STORAGE_AZURE_PREFIX`|Azure Storage prefix to use for the registry (optional)|
|`--storage-azure-signedurl-expiry`|`BORING_REGISTRY_STORAGE_AZURE_SIGNEDURL_EXPIRY`|Generate Azure Storage signed URL valid for X seconds. (default 5m0s)|

The following shows a minimal example to run `boring-registry server` with Azure Blob Storage:

```console
$ boring-registry server \
  --storage-azure-account=boring-registry \
  --storage-azure-container=boring-registry
```
