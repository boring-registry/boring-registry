# ARG before first stage to share the value across multiple stages
ARG BASEDIR=/go/src/github.com/boring-registry/boring-registry

FROM golang:1.21 AS build

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIMESTAMP
ARG BASEDIR # use the default value

WORKDIR ${BASEDIR}

COPY . ${BASEDIR}
RUN CGO_ENABLED=0 go build -ldflags "-s -w \
    -X github.com/boring-registry/boring-registry/version.Version=${VERSION} \
    -X github.com/boring-registry/boring-registry/version.Commit=${GIT_COMMIT} \
    -X github.com/boring-registry/boring-registry/version.Date=${BUILD_TIMESTAMP}"

FROM gcr.io/distroless/base:nonroot

ARG BASEDIR
COPY --from=build ${BASEDIR}/boring-registry /

ENTRYPOINT ["/boring-registry"]
CMD ["server"]
