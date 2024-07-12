# Storage Layout

The boring-registry is using the following storage layout inside the storage backend:

```console
<bucket_prefix>
├── modules
│   └── <namespace>
│       └── <name>
│           └── <provider>
│               ├── <namespace>-<name>-<provider>-<version>.tar.gz
│               └── <namespace>-<name>-<provider>-<version>.tar.gz
├── providers
│   └── <namespace>
│       ├── signing-keys.json
│       └── <name>
│           ├── terraform-provider-<name>_<version>_SHA256SUMS
│           ├── terraform-provider-<name>_<version>_SHA256SUMS.sig
│           └── terraform-provider-<name>_<version>_<os>_<arch>.zip
└── mirror
    └── providers
        └── <hostname>
            └── <namespace>
                ├── signing-keys.json
                └── <name>
                    ├── terraform-provider-<name>_<version>_SHA256SUMS
                    ├── terraform-provider-<name>_<version>_SHA256SUMS.sig
                    └── terraform-provider-<name>_<version>_<os>_<arch>.zip
```

The `<bucket_prefix>` is an optional prefix under which the boring-registry storage is organized and can be set with the `--storage-s3-prefix` or `--storage-gcs-prefix` flags.

An example without any placeholders could be the following:

```console
<bucket_prefix>
├── modules
│   └── acme
│       └── tls-private-key
│           └── aws
│               ├── acme-tls-private-key-aws-0.1.0.tar.gz
│               └── acme-tls-private-key-aws-0.2.0.tar.gz
├── providers
│   └── acme
│       ├── signing-keys.json
│       └── dummy
│           ├── terraform-provider-dummy_0.1.0_SHA256SUMS
│           ├── terraform-provider-dummy_0.1.0_SHA256SUMS.sig
│           ├── terraform-provider-dummy_0.1.0_linux_amd64.zip
│           └── terraform-provider-dummy_0.1.0_linux_arm64.zip
└── mirror
    └── providers
        └── terraform.example.com
            └── acme
                ├── signing-keys.json
                └── random
                    ├── terraform-provider-random_0.1.0_SHA256SUMS
                    ├── terraform-provider-random_0.1.0_SHA256SUMS.sig
                    └── terraform-provider-random_0.1.0_linux_amd64.zip
```
