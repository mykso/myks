# Stage 1 Build myks
FROM --platform=$BUILDPLATFORM golang:1.21 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app

COPY . .

RUN go mod download \
 && go mod verify

RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -o myks main.go


# Stage 2 Download tools
FROM --platform=$BUILDPLATFORM debian:bookworm AS download-tools

ARG TARGETOS
ARG TARGETARCH
# renovate: datasource=github-releases depName=helm/helm
ARG HELM_VERSION=3.11.2
# renovate: datasource=github-releases depName=carvel-dev/vendir
ARG VENDIR_VERSION=v0.38.0
# renovate: datasource=github-releases depName=carvel-dev/ytt
ARG YTT_VERSION=v0.46.3

RUN apt-get update \
 && apt-get install --no-install-recommends -y \
      ca-certificates \
      curl \
      unzip

WORKDIR /tools

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN curl -fsSL \
      https://get.helm.sh/helm-v${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz \
    | tar -xzf - --strip-components=1 ${TARGETOS}-${TARGETARCH}/helm
RUN curl -fsSL \
      https://github.com/vmware-tanzu/carvel-vendir/releases/download/v${VENDIR_VERSION}/vendir-${TARGETOS}-${TARGETARCH} \
    > vendir
RUN curl -fsSL \
      https://github.com/vmware-tanzu/carvel-ytt/releases/download/v${YTT_VERSION}/ytt-${TARGETOS}-${TARGETARCH} \
    > ytt
RUN chmod +x *


# Stage 3: Bring it all together
FROM --platform=$BUILDPLATFORM debian:bookworm

WORKDIR /app

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

WORKDIR /app

ENTRYPOINT ["tini", "--", "myks"]
