FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS downloader
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
WORKDIR /tools
RUN apt-get update \
 && apt-get install --no-install-recommends -y \
      ca-certificates \
      curl \
      unzip \
 && rm -rf /var/lib/apt/lists/*


FROM downloader AS helm
ARG TARGETOS
ARG TARGETARCH
# renovate: datasource=github-releases depName=helm/helm
ARG HELM_VERSION=v3.14.3
RUN curl -fsSL \
      https://get.helm.sh/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz \
    | tar -xzf - --strip-components=1 ${TARGETOS}-${TARGETARCH}/helm


FROM downloader AS vendir
ARG TARGETOS
ARG TARGETARCH
# renovate: datasource=github-releases depName=carvel-dev/vendir
ARG VENDIR_VERSION=v0.40.0
RUN curl -fsSL \
      https://github.com/carvel-dev/vendir/releases/download/${VENDIR_VERSION}/vendir-${TARGETOS}-${TARGETARCH} \
    > vendir


FROM downloader AS ytt
ARG TARGETOS
ARG TARGETARCH
# renovate: datasource=github-releases depName=carvel-dev/ytt
ARG YTT_VERSION=v0.48.0
RUN curl -fsSL \
      https://github.com/carvel-dev/ytt/releases/download/${YTT_VERSION}/ytt-${TARGETOS}-${TARGETARCH} \
    > ytt


FROM --platform=$BUILDPLATFORM debian:bookworm-slim

RUN apt-get update \
 && apt-get install --no-install-recommends -y \
      ca-certificates \
      git \
      gnupg2 \
      ssh \
      tini \
 && rm -rf /var/lib/apt/lists/*

COPY --link --chmod=700 --from=helm /tools/helm /usr/local/bin/
COPY --link --chmod=700 --from=vendir /tools/vendir /usr/local/bin/
COPY --link --chmod=700 --from=ytt /tools/ytt /usr/local/bin/
# This copies myks binary built by goreleaser
COPY --link myks /usr/local/bin/

WORKDIR /app

ENTRYPOINT ["tini", "--", "myks"]
