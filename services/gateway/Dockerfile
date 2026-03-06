FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY infra/ infra/
COPY services/gateway/ services/gateway/
RUN cd services/gateway && go build -o /gateway ./cmd/main.go

FROM alpine:3.20
COPY --from=builder /gateway /gateway
EXPOSE 8080
CMD ["/gateway"]
