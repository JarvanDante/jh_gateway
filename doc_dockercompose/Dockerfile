FROM golang:1.25.3-alpine AS builder

WORKDIR /app

COPY ../go.mod go.sum ./
RUN go mod download

COPY .. .
RUN CGO_ENABLED=0 GOOS=linux go build -o jh_gateway .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/jh_gateway .
COPY ../config.yaml .

EXPOSE 8901

CMD ["./jh_gateway"]
