# Download Proxy

By default the boring-registry returns pre-signed URLs pointing to the remote storage as download URLs for the Terraform CLI.
The boring-registry can be configured to proxy the files from the storage backend and serve them directly from the boring-registry.

You can activate the download proxy by using the `--download-proxy` flag or by setting the `BORING_REGISTRY_DOWNLOAD_PROXY=true` environment variable.

***Note :** If activated, the download proxy functionality will be applied to modules and providers, but not mirrors.*
