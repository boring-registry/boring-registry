# Google Cloud Storage

## Authorization

Make sure the server has valid Google Cloud credentials set.
Check the [official documentation](https://cloud.google.com/sdk/docs/authorizing) for the supported authorization methods.

## Configuration for Google Cloud Storage

The following configuration options are available:

|Flag|Environment Variable|Description|
|---|---|---|
|`--storage-gcs-bucket`|`BORING_REGISTRY_STORAGE_GCS_BUCKET`|Bucket to use when using the GCS registry type|
|`--storage-gcs-prefix`|`BORING_REGISTRY_STORAGE_GCS_PREFIX`|Prefix to use when using the GCS registry type (optional)|
|`--storage-gcs-sa-email string`|`BORING_REGISTRY_STORAGE_GCS_SA_EMAIL`|Google service account email to be used for Application Default Credentials (ADC) (optional)|
|`--storage-gcs-signedurl-expiry`|`BORING_REGISTRY_STORAGE_GCS_SIGNEDURL_EXPIRY`|Generate GCS Storage signed URL valid for X seconds. (default 30s)|

The following shows a minimal example to run `boring-registry server` with Google Cloud Storage:

```console
$ boring-registry server \
  --storage-gsc-bucket=boring-registry
```
