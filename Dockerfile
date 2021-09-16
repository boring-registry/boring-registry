FROM golang:1.16 AS build

ENV BASEDIR /go/src/github.com/TierMobility/boring-registry

WORKDIR ${BASEDIR}

ADD . ${BASEDIR}

RUN go mod vendor
RUN go install -mod=vendor github.com/TierMobility/boring-registry/cmd/boring-registry/...

FROM gcr.io/distroless/base:nonroot

COPY --from=build /go/bin/boring-registry /

ENTRYPOINT ["/boring-registry", "server"]
