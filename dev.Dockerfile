# Stage 1 Build myks
FROM --platform=$BUILDPLATFORM golang:1.25.5@sha256:68ee6dfa91a074c6e62a8af3ff0a427a36a78015778247af4f18398f37a5e91b AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app

COPY . .

RUN go mod download \
 && go mod verify

RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -o myks main.go


# Stage 2 Download tools
FROM --platform=$BUILDPLATFORM debian:trixie@sha256:0d01188e8dd0ac63bf155900fad49279131a876a1ea7fac917c62e87ccb2732d AS download-tools

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
FROM --platform=$BUILDPLATFORM debian:trixie@sha256:0d01188e8dd0ac63bf155900fad49279131a876a1ea7fac917c62e87ccb2732d

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
