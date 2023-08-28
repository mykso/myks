# Stage 1 Build myks
FROM golang:1.20 AS builder
COPY . /app
WORKDIR /app
RUN go mod download && \
    go mod verify
RUN CGO_ENABLED=0 \
    go build -trimpath -o myks main.go

# Stage 2 Download tools
FROM debian:bookworm AS download-tools
ARG HELM_VERSION=3.11.2
ARG VENDIR_VERSION=0.33.1
ARG YTT_VERSION=0.45.0
RUN apt-get update \
  && apt-get install --no-install-recommends -y \
  ca-certificates \
  curl \
  unzip
ENV TOOLS_ROOT=/tools
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN mkdir ${TOOLS_ROOT} && \
  curl -fsSL \
  https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz | \
  tar -xzf - -C ${TOOLS_ROOT} --strip-components=1 linux-amd64/helm && \
  curl -fsSL \
  https://github.com/vmware-tanzu/carvel-vendir/releases/download/v${VENDIR_VERSION}/vendir-linux-amd64 \
  > ${TOOLS_ROOT}/vendir && \
  curl -fsSL \
  https://github.com/vmware-tanzu/carvel-ytt/releases/download/v${YTT_VERSION}/ytt-linux-amd64 \
  > ${TOOLS_ROOT}/ytt
RUN chmod +x ${TOOLS_ROOT}/*

# Stage 3: Bring it all together
FROM debian:bookworm
RUN apt-get update \
  && apt-get install --no-install-recommends -y \
  ca-certificates \
  git \
  gnupg2 \
  ssh \
  tini && \
  rm -rf /var/lib/apt/lists/*
ENV TOOLS_ROOT=/tools
WORKDIR /app
ENV PATH="${PATH}:${TOOLS_ROOT}"
COPY --from=download-tools ${TOOLS_ROOT} ${TOOLS_ROOT}
COPY --from=builder app/myks ${TOOLS_ROOT}
ENTRYPOINT ["tini", "--", "myks"]
