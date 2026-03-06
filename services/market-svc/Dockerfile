FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY infra/ infra/
COPY services/market-svc/ services/market-svc/
RUN cd services/market-svc && go build -o /market-svc ./cmd/main.go

FROM alpine:3.20
COPY --from=builder /market-svc /market-svc
EXPOSE 9002
CMD ["/market-svc"]
