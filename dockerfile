FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /booking-service

COPY app/go.mod /booking-service/

RUN go mod download

COPY app/ /booking-service/

RUN go build -o build/main cmd/main.go

FROM alpine:latest AS runner

WORKDIR /app

COPY --from=builder /booking-service/build/main /app/
COPY /config.yaml /app/config.yaml
COPY /migrations /app/migrations

ENV CONFIG_PATH=/app/config.yaml
ENV APP_MIGRATION_DIR=/app/migrations

CMD [ "/app/main" ]