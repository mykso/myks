# Stage 1 Build myks
FROM golang:1.21 AS builder

WORKDIR /app

COPY . .

RUN go mod download \
 && go mod verify

RUN CGO_ENABLED=0 \
    go build -trimpath -o myks main.go


# Stage 2 Download tools
FROM debian:bookworm AS download-tools

RUN apt-get update \
 && apt-get install --no-install-recommends -y \
      ca-certificates \
      curl \
      unzip

ARG HELM_VERSION=3.11.2
ARG VENDIR_VERSION=0.33.1
ARG YTT_VERSION=0.45.0

WORKDIR /tools

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN curl -fsSL \
      https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz \
    | tar -xzf - --strip-components=1 linux-amd64/helm
RUN curl -fsSL \
      https://github.com/vmware-tanzu/carvel-vendir/releases/download/v${VENDIR_VERSION}/vendir-linux-amd64 \
    > vendir
RUN curl -fsSL \
      https://github.com/vmware-tanzu/carvel-ytt/releases/download/v${YTT_VERSION}/ytt-linux-amd64 \
    > ytt
RUN chmod +x *


# Stage 3: Bring it all together
FROM debian:bookworm

RUN apt-get update \
 && apt-get install --no-install-recommends -y \
      ca-certificates \
      git \
      gnupg2 \
      ssh \
      tini \
 && rm -rf /var/lib/apt/lists/*

COPY --from=download-tools /tools/* /usr/local/bin/
COPY --from=builder /app/myks /usr/local/bin/

ENTRYPOINT ["tini", "--", "myks"]
