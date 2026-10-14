############################
# 1) Build stage
############################
FROM --platform=$BUILDPLATFORM golang:1.25.3-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata git

ARG TARGETOS=linux
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOPROXY=https://proxy.golang.org,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p pkg && go build -trimpath -ldflags="-s -w" -o pkg/jh_gateway .

############################
# 2) Runtime stage
############################
FROM alpine:3.20

# 统一运行目录到 /app
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app \
    && mkdir -p /app/config /app/manifest/config

COPY --from=builder /src/pkg/jh_gateway /app/jh_gateway

# 默认配置（生产建议用 volume 覆盖 /app/config.yaml）
COPY config.yaml /app/config.yaml
# 兼容一些默认搜索路径
COPY config.yaml /app/config/config.yaml
COPY config.yaml /app/manifest/config/config.yaml

EXPOSE 8100 8003

USER app
CMD ["/app/jh_gateway"]
