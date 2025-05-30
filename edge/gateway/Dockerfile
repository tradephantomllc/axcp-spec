FROM golang:1.22 as build
WORKDIR /src
COPY . .
RUN cd edge/gateway && go build -o /out/axcp-gateway ./cmd/gateway

FROM debian:bookworm-slim
COPY --from=build /out/axcp-gateway /usr/bin/axcp-gateway
ENTRYPOINT ["axcp-gateway"]
