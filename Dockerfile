# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/vylo-server ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.20
# ca-certificates: outbound HTTPS to vylo.cc; wget: healthcheck
RUN adduser -D -u 10001 vylo && apk add --no-cache wget ca-certificates
USER vylo
COPY --from=build /out/vylo-server /usr/local/bin/vylo-server
ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/healthz" || exit 1
ENTRYPOINT ["vylo-server"]
