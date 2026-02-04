# Stage 1 Build myks
FROM --platform=$BUILDPLATFORM golang:1.25.7@sha256:011d6e21edbc198b7aeb06d705f17bc1cc219e102c932156ad61db45005c5d31 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app

COPY . .

RUN go mod download \
 && go mod verify

RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -o myks main.go


# Stage 2 Download tools
FROM --platform=$BUILDPLATFORM debian:trixie@sha256:2c91e484d93f0830a7e05a2b9d92a7b102be7cab562198b984a84fdbc7806d91 AS download-tools

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
FROM --platform=$BUILDPLATFORM debian:trixie@sha256:2c91e484d93f0830a7e05a2b9d92a7b102be7cab562198b984a84fdbc7806d91

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
