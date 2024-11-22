ARG GCR_MIRROR=gcr.io/
FROM ${GCR_MIRROR}distroless/static:nonroot
LABEL org.opencontainers.image.source https://github.com/norskhelsenett/ror-ms-tanzu-clidownloader
WORKDIR /

COPY --chown=1001:1001  dist/ms-tanzu-clidownloader /bin/clidownloader
ENTRYPOINT ["/bin/clidownloader"]
