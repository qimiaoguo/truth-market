# ──────────────────────────────────────────────────
# Stage 1: Build the Go service binary
# ──────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

ARG SERVICE=gateway

# Disable go.work so each module uses its own go.mod/replace directives.
ENV GOWORK=off

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy shared modules (referenced via replace directives in each service's go.mod)
COPY pkg/ pkg/
COPY infra/ infra/
COPY proto/gen/go/ proto/gen/go/

# Copy the target service
COPY services/${SERVICE}/ services/${SERVICE}/

# Download dependencies for each module that is present.
RUN cd pkg && go mod download
RUN cd infra && go mod download
RUN cd proto/gen/go && go mod download
RUN cd services/${SERVICE} && go mod download

# Build the service binary
RUN cd services/${SERVICE} && \
    CGO_ENABLED=0 \
    go build -ldflags="-s -w" -o /server ./cmd/

# ──────────────────────────────────────────────────
# Stage 2: Minimal runtime image
# ──────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup

COPY --from=builder /server /server

USER appuser

EXPOSE 8080

ENTRYPOINT ["/server"]
