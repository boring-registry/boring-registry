FROM golang:1.20 AS build

ENV BASEDIR /go/src/github.com/TierMobility/boring-registry

WORKDIR ${BASEDIR}

ADD . ${BASEDIR}

RUN CGO_ENABLED=0 go install -mod=vendor github.com/TierMobility/boring-registry

FROM gcr.io/distroless/base:nonroot

COPY --from=build ${BASEDIR}/registry /

ENTRYPOINT ["/boring-registry", "server"]
