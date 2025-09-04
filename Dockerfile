# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.22 AS debug
ARG TARGETARCH
ARG SERVICE_NAME
ENV TARGETARCH=$TARGETARCH
ENV SERVICE_NAME=$SERVICE_NAME

RUN go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /workspace
COPY --chown=65532:65532 --chmod=0555 bin/$SERVICE_NAME-$TARGETARCH /workspace/app
USER 65532:65532

ENTRYPOINT ["/go/bin/dlv", "exec", "--headless", "-l", ":10000", "--api-version=2", "/workspace/app", "--"]

FROM gcr.io/distroless/base:nonroot AS prod
ARG TARGETARCH
ARG SERVICE_NAME
ENV TARGETARCH=$TARGETARCH
ENV SERVICE_NAME=$SERVICE_NAME

WORKDIR /workspace
COPY --chown=65532:65532 --chmod=0555 bin/$SERVICE_NAME-$TARGETARCH /workspace/app
COPY --chown=65532:65532 NOTICE /workspace/NOTICE

USER 65532:65532

ENTRYPOINT ["/workspace/app"]
