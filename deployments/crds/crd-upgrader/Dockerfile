FROM registry.access.redhat.com/ubi9/ubi-minimal
ARG TARGETARCH
ENV TARGETARCH=$TARGETARCH
RUN curl -o kubectl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$TARGETARCH/kubectl" \
&& chmod +x kubectl && mv kubectl /usr/bin/kubectl

COPY deployments/crds/definitions /crds
RUN curl -L https://raw.githubusercontent.com/kubernetes-sigs/kueue/refs/tags/v0.12.2/charts/kueue/templates/crd/kueue.x-k8s.io_topologies.yaml | \
    sed -e '/^  labels:/,/}}/d' -e '/^  annotations:/,/v0.17.3/d' -e '/^  conversion:/,/v1/d' | \
    sed 's/^metadata:/metadata:\n  labels:\n    kai-scheduler-installed: "true"/' > /crds/kueue.x-k8s.io_topologies.yaml
RUN cat /crds/kueue.x-k8s.io_topologies.yaml
COPY NOTICE .

USER 65532:65532

# Using --force-conflicts to claim ownership of the CRDs from helm
CMD ["kubectl", "apply", "--server-side=true", "--force-conflicts", "-f", "/crds"]