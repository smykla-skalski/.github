# syntax=docker/dockerfile:1@sha256:4a43a54dd1fedceb30ba47e76cfcf2b47304f4161c0caeac2db1c61804ea3c91

# Use BUILDPLATFORM to run apk on native arch (avoids QEMU issues on cross-build)
FROM --platform=$BUILDPLATFORM alpine:latest@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS certs
RUN apk --no-cache add ca-certificates

FROM scratch

ARG TARGETPLATFORM

LABEL org.opencontainers.image.source="https://github.com/smykla-skalski/.github"
LABEL org.opencontainers.image.description="Organization sync tool for labels, files, and smyklot versions"
LABEL org.opencontainers.image.licenses="MIT"

# Copy CA certificates from alpine
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy pre-built binary from GoReleaser to /usr/local/bin for PATH access
# TARGETPLATFORM is set by docker buildx (e.g., linux/amd64, linux/arm64)
COPY ${TARGETPLATFORM}/dotsync /usr/local/bin/dotsync

# Set PATH to include /usr/local/bin (scratch image has no PATH by default)
ENV PATH="/usr/local/bin:${PATH}"

ENTRYPOINT ["dotsync"]
