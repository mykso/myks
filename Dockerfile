FROM --platform=$BUILDPLATFORM debian:bookworm-slim@sha256:2424c1850714a4d94666ec928e24d86de958646737b1d113f5b2207be44d37d8 AS downloader
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
ARG HELM_VERSION=v3.17.3
RUN curl -fsSL \
      https://get.helm.sh/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz \
    | tar -xzf - --strip-components=1 ${TARGETOS}-${TARGETARCH}/helm



FROM --platform=$BUILDPLATFORM debian:bookworm-slim@sha256:2424c1850714a4d94666ec928e24d86de958646737b1d113f5b2207be44d37d8

RUN apt-get update \
 && apt-get install --no-install-recommends -y \
      ca-certificates \
      git \
      gnupg2 \
      ssh \
      tini \
 && rm -rf /var/lib/apt/lists/*

COPY --link --chmod=700 --from=helm /tools/helm /usr/local/bin/
# This copies myks binary built by goreleaser
COPY --link myks /usr/local/bin/

WORKDIR /app

ENTRYPOINT ["tini", "--", "myks"]
