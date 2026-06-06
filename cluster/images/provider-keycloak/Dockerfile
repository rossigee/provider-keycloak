FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
ADD _output/linux_amd64/provider /bin/provider
ADD package/crds /crds
ADD package/crossplane.yaml /package.yaml
ENTRYPOINT ["/bin/provider"]