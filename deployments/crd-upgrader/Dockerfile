# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

FROM registry.access.redhat.com/ubi9/ubi-minimal AS builder
ARG TARGETARCH
ENV TARGETARCH=$TARGETARCH
RUN curl -o kubectl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$TARGETARCH/kubectl" \
&& chmod +x kubectl

FROM nvcr.io/nvidia/distroless/go:v3.2.1

COPY --from=builder --chmod=0755 kubectl /usr/bin/kubectl
COPY --chmod=0644 deployments/kai-scheduler/crds /internal-crds
COPY --chmod=0644 deployments/external-crds /external-crds
COPY --chmod=0644 NOTICE .
COPY --chmod=0755 deployments/crd-upgrader/apply-crds.sh /apply-crds.sh

USER 65532:65532

CMD ["/busybox/sh","/apply-crds.sh"]