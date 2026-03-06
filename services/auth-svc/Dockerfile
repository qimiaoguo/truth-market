FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY infra/ infra/
COPY services/auth-svc/ services/auth-svc/
RUN cd services/auth-svc && go build -o /auth-svc ./cmd/main.go

FROM alpine:3.20
COPY --from=builder /auth-svc /auth-svc
EXPOSE 9001
CMD ["/auth-svc"]
