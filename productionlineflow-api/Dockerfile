FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY config /app/config
EXPOSE 8080
CMD ["/app/api"]
