# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

# Use BUILDPLATFORM to run apk on native arch (avoids QEMU issues on cross-build)
FROM --platform=$BUILDPLATFORM alpine:latest@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS certs
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
