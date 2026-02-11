# Build the manager binary
ARG GOLANG_VERSION=1.25.5-1770654497

ARG BUILDPLATFORM TARGETPLATFORM

# These built-in args are defined in the global scope, and are not automatically accessible within build stages or RUN commands.Add commentMore actions
# To expose these arguments inside the build stage, we need to redefine it without a value.
ARG TARGETOS TARGETARCH

################################################################################
FROM --platform=$BUILDPLATFORM registry.access.redhat.com/ubi9/toolbox as manifests
ARG USE_LOCAL=false
ARG OVERWRITE_MANIFESTS=""
USER root
WORKDIR /
COPY opt/manifests/ /opt/manifests/
COPY get_all_manifests.sh get_all_manifests.sh
RUN if [ "${USE_LOCAL}" != "true" ]; then \
        rm -rf /opt/manifests/*; \
        ./get_all_manifests.sh ${OVERWRITE_MANIFESTS}; \
    fi

# Clean up unwanted directories and files from manifests
RUN rm -rf /opt/manifests/*/e2e /opt/manifests/*/scorecard /opt/manifests/*/test /opt/manifests/*/samples /opt/manifests/*/example-* \
    && find /opt/manifests -name "README.md" -delete

# Copy monitoring config
COPY config/monitoring/ /opt/manifests/monitoring
# Copy ods-configs
COPY config/osd-configs/ /opt/manifests/osd-configs

################################################################################
FROM --platform=$BUILDPLATFORM registry.access.redhat.com/ubi9/go-toolset:$GOLANG_VERSION as builder
ARG CGO_ENABLED=1
ARG TARGETOS TARGETARCH
USER root
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY api/ api/
COPY internal/ internal/
COPY cmd/main.go cmd/main.go
COPY pkg/ pkg/

RUN echo "GOOS=${TARGETOS} GOARCH=${TARGETARCH}"

# Build stripe out debug info to minimize binary size
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -ldflags="-s -w" -tags strictfipsruntime -o manager cmd/main.go

################################################################################
FROM --platform=$TARGETPLATFORM registry.access.redhat.com/ubi9/ubi-minimal:latest
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --chown=1001:0 --from=manifests /opt/manifests /opt/manifests
# Recursive change all files
RUN chown -R 1001:0 /opt/manifests &&\
    chmod -R g=u /opt/manifests
USER 1001

ENTRYPOINT ["/manager"]
