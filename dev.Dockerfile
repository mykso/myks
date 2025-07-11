# Stage 1 Build myks
FROM --platform=$BUILDPLATFORM golang:1.24.5@sha256:14fd8a55e59a560704e5fc44970b301d00d344e45d6b914dda228e09f359a088 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app

COPY . .

RUN go mod download \
 && go mod verify

RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -o myks main.go


# Stage 2 Download tools
FROM --platform=$BUILDPLATFORM debian:bookworm@sha256:d42b86d7e24d78a33edcf1ef4f65a20e34acb1e1abd53cabc3f7cdf769fc4082 AS download-tools

ARG TARGETOS
ARG TARGETARCH
# renovate: datasource=github-releases depName=helm/helm
ARG HELM_VERSION=v3.14.3

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
RUN chmod +x *


# Stage 3: Bring it all together
FROM --platform=$BUILDPLATFORM debian:bookworm@sha256:d42b86d7e24d78a33edcf1ef4f65a20e34acb1e1abd53cabc3f7cdf769fc4082

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
