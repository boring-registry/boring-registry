# AWS S3

## Authorization

Make sure the boring-registry has valid AWS credentials set which are authorized to access the S3 bucket.
This can for example be achieved by setting the `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables.

More information on this topic can be found in the [official documentation by AWS](https://docs.aws.amazon.com/sdkref/latest/guide/creds-config-files.html).

## Configuration for S3

The following configuration options are available:

|Flag|Environment Variable|Description|
|---|---|---|
|`--storage-s3-bucket`|`BORING_REGISTRY_STORAGE_S3_BUCKET`|S3 bucket to use for the registry|
|`--storage-s3-endpoint`|`BORING_REGISTRY_STORAGE_S3_ENDPOINT`|S3 bucket endpoint URL (optional)|
|`--storage-s3-pathstyle`|`BORING_REGISTRY_STORAGE_S3_PATHSTYLE`|S3 use PathStyle (optional)|
|`--storage-s3-prefix`|`BORING_REGISTRY_STORAGE_S3_PREFIX`|S3 bucket prefix to use for the registry (optional)|
|`--storage-s3-region`|`BORING_REGISTRY_STORAGE_S3_REGION` or `AWS_REGION` or `AWS_DEFAULT_REGION`|S3 bucket region to use for the registry|
|`--storage-s3-signedurl-expiry`|`BORING_REGISTRY_STORAGE_S3_SIGNEDURL_EXPIRY`|Generate S3 signed URL valid for X seconds (default 5m0s)|

The following shows a minimal example to run `boring-registry server` with S3:

```console
$ boring-registry server \
  --storage-s3-bucket=boring-registry \
  --storage-s3-region=us-east-1
```

