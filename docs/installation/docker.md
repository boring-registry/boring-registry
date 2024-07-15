# Installing with Docker

Container images are published to [`ghcr.io/boring-registry/boring-registry`](https://github.com/boring-registry/boring-registry/pkgs/container/boring-registry) for every tagged release of the project.

Containers can be started with any container engine as demonstrated with `docker` in the following:

```console
$ docker pull ghcr.io/boring-registry/boring-registry:latest

# Start boring-registry with 'server --help' CLI arguments
$ docker run -p 5601:5601 ghcr.io/boring-registry/boring-registry:latest server --help
```
