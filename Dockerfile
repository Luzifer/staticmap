FROM golang:alpine as builder

COPY . /go/src/github.com/Luzifer/staticmap
WORKDIR /go/src/github.com/Luzifer/staticmap

RUN set -ex \
 && apk add --update git \
 && go install \
      -ldflags "-X main.version=$(git describe --tags --always || echo dev)" \
      -mod=readonly

FROM alpine:latest

ENV CACHE_DIR /data/map-cache
ENV XDG_CACHE_HOME /data/tile-cache

LABEL maintainer "Knut Ahlers <knut@ahlers.me>"

RUN set -ex \
 && apk --no-cache add \
      ca-certificates

COPY --from=builder /go/bin/staticmap /usr/local/bin/staticmap

EXPOSE 3000
VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/staticmap"]
CMD ["--"]

# vim: set ft=Dockerfile:
