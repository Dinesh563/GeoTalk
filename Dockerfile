# syntax=docker/dockerfile:1

FROM golang:1.23.0-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/app .

FROM alpine:latest  
WORKDIR /root/
COPY --from=builder /out/app .
CMD ["./app"]