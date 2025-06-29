# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

FROM registry.access.redhat.com/ubi9/ubi-minimal
ARG TARGETARCH
ENV TARGETARCH=$TARGETARCH
RUN curl -o kubectl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$TARGETARCH/kubectl" \
&& chmod +x kubectl && mv kubectl /usr/bin/kubectl

COPY deployments/crds/definitions /crds
RUN mkdir -p /external-crds
# Download the kueue topology crd definitions, and remove all kueue helm installation parameters
RUN curl -L https://raw.githubusercontent.com/kubernetes-sigs/kueue/refs/tags/v0.12.2/charts/kueue/templates/crd/kueue.x-k8s.io_topologies.yaml | \
    sed -e '/^  labels:/,/}}/d' -e '/^    {{- if/,/end }}/d' -e '/^  conversion:/,/v1/d' > /external-crds/kueue.x-k8s.io_topologies.yaml
COPY NOTICE .
COPY deployments/crds/crd-upgrader/apply-crds.sh /apply-crds.sh
RUN chmod +x /apply-crds.sh

USER 65532:65532

CMD ["/apply-crds.sh"]