FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY infra/ infra/
COPY services/trading-svc/ services/trading-svc/
RUN cd services/trading-svc && go build -o /trading-svc ./cmd/main.go

FROM alpine:3.20
COPY --from=builder /trading-svc /trading-svc
EXPOSE 9003
CMD ["/trading-svc"]
