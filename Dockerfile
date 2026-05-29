FROM gcr.io/distroless/static:nonroot@sha256:963fa6c544fe5ce420f1f54fb88b6fb01479f054c8056d0f74cc2c6000df5240
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/azure-alerts-proxy /app/azure-alerts-proxy

EXPOSE 8080
USER 65532
ENTRYPOINT ["/app/azure-alerts-proxy"]
