# syntax=docker/dockerfile:1

# Use BUILDPLATFORM to run apk on native arch (avoids QEMU issues on cross-build)
FROM --platform=$BUILDPLATFORM alpine:latest AS certs
RUN apk --no-cache add ca-certificates

FROM scratch

ARG TARGETPLATFORM

LABEL org.opencontainers.image.source="https://github.com/smykla-labs/.github"
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
