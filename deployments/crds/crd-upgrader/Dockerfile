# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

FROM registry.access.redhat.com/ubi9/ubi-minimal
ARG TARGETARCH
ENV TARGETARCH=$TARGETARCH
RUN curl -o kubectl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$TARGETARCH/kubectl" \
&& chmod +x kubectl && mv kubectl /usr/bin/kubectl


COPY deployments/crds/internal /internal-crds
COPY deployments/crds/external /external-crds
COPY NOTICE .
COPY deployments/crds/crd-upgrader/apply-crds.sh /apply-crds.sh
RUN chmod +x /apply-crds.sh

USER 65532:65532

CMD ["/apply-crds.sh"]