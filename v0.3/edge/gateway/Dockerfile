# SPDX-License-Identifier: Apache-2.0
# Placeholder – v0.3 structure only
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/gateway .
EXPOSE 7143
CMD ["./gateway"]
