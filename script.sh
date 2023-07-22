docker build -t ghcr.io/tiermobility/boring-registry:v0.11.0-custom .
docker tag ghcr.io/tiermobility/boring-registry:v0.11.0-custom us-east4-docker.pkg.dev/dps-services/boring-registry/boring-registry:v0.11.0-custom
docker push us-east4-docker.pkg.dev/dps-services/boring-registry/boring-registry:v0.11.0-custom