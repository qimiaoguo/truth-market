# ──────────────────────────────────────────────────
# Stage 1: Build the Go service binary
# ──────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

ARG SERVICE=gateway

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy workspace-level files
COPY go.work go.work.sum ./

# Copy shared modules
COPY pkg/ pkg/
COPY infra/ infra/
COPY proto/gen/go/ proto/gen/go/

# Copy the target service
COPY services/${SERVICE}/ services/${SERVICE}/

# Download dependencies for all workspace modules that are present.
# We run go mod download from each module directory so the workspace
# resolver can satisfy cross-module requires.
RUN cd pkg && go mod download
RUN cd infra && go mod download
RUN cd proto/gen/go && go mod download
RUN cd services/${SERVICE} && go mod download

# Build the service binary
RUN cd services/${SERVICE} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /server ./cmd/main.go

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
