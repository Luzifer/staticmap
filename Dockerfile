FROM golang:alpine

LABEL maintainer "Knut Ahlers <knut@ahlers.me>"

ENV CACHE_DIR /data/map-cache
ENV XDG_CACHE_HOME /data/tile-cache

ADD . /go/src/github.com/Luzifer/staticmap
WORKDIR /go/src/github.com/Luzifer/staticmap

RUN set -ex \
 && apk add --update git ca-certificates \
 && go install -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)" \
 && apk del --purge git

EXPOSE 3000

VOLUME ["/data"]

ENTRYPOINT ["/go/bin/staticmap"]
CMD ["--"]
