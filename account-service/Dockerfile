FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY EXBanka-3-Backend/account-service/ ./EXBanka-3-Backend/account-service/
WORKDIR /app/EXBanka-3-Backend/account-service
RUN go build -o /account-service ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /account-service ./account-service
EXPOSE 8084 9094
CMD ["./account-service"]
