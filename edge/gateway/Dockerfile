FROM golang:1.25.6-bookworm AS build
WORKDIR /src
COPY go.mod go.sum go.work go.work.sum ./
COPY proto ./proto
COPY sdk/go ./sdk/go
COPY edge/gateway ./edge/gateway
RUN cd edge/gateway && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/axcp-gateway ./cmd/gateway

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/axcp-gateway /usr/local/bin/axcp-gateway
USER 65532:65532
EXPOSE 7143/udp
ENTRYPOINT ["/usr/local/bin/axcp-gateway"]
