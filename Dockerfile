# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder

WORKDIR /src

RUN apk add --no-cache build-base git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/cicd ./cmd/cicd

FROM alpine:3.20

RUN apk add --no-cache ca-certificates git openssh-client curl kubectl && update-ca-certificates

WORKDIR /app

COPY --from=builder /out/cicd /app/cicd
COPY web /app/web
COPY config /app/config

RUN mkdir -p /app/data /app/work

EXPOSE 8080

ENTRYPOINT ["/app/cicd"]
CMD ["-config", "/app/config/apps.yaml", "-db", "/app/data/cicd.db", "-work", "/app/work", "-static", "/app/web", "-addr", ":8080"]
