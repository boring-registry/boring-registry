# Installing with Helm

The project provides a Helm chart as the supported method of installation for Kubernetes.


To install the `boring-registry` Helm chart, use the upgrade command as shown below:

```console
helm upgrade \
  --install \
  --wait \
  --namespace boring-registry \
  --create-namespace \
  boring-registry \
  oci://ghcr.io/boring-registry/charts/boring-registry
```

Check [`ghcr.io/boring-registry/charts/boring-registry`](https://github.com/boring-registry/boring-registry/pkgs/container/charts%2Fboring-registry) for all available versions.
