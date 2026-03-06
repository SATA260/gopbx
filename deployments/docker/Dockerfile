FROM golang:1.25 AS builder
WORKDIR /src
COPY . .
RUN go build -o /out/gopbx ./cmd/gateway

FROM debian:stable-slim
WORKDIR /app
COPY --from=builder /out/gopbx /app/gopbx
EXPOSE 8080
CMD ["/app/gopbx"]
