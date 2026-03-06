FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY infra/ infra/
COPY services/ranking-svc/ services/ranking-svc/
RUN cd services/ranking-svc && go build -o /ranking-svc ./cmd/main.go

FROM alpine:3.20
COPY --from=builder /ranking-svc /ranking-svc
EXPOSE 9004
CMD ["/ranking-svc"]
