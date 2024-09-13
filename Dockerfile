FROM ubuntu:latest

# copy binary from builder
WORKDIR /
COPY registry-cache /usr/bin/registry-cache

ENTRYPOINT ["/usr/bin/registry-cache"]