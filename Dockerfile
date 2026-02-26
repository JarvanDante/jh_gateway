
############################
# 1) Build stage
############################
FROM golang:1.25.3-alpine AS builder

WORKDIR /src

# 证书/时区/构建依赖
RUN apk add --no-cache ca-certificates tzdata git

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY=https://proxy.golang.org,direct

# 先拷贝依赖清单，加速缓存
COPY go.mod go.sum ./
RUN go mod download

# 再拷贝源码并编译
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/jh_gateway .

############################
# 2) Runtime stage
############################
FROM alpine:3.20

WORKDIR /app

# 运行时建议保留证书和时区
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app

COPY --from=builder /out/jh_gateway /app/jh_gateway
# 配置文件也可以改成通过 ConfigMap 挂载，这里先保留镜像内默认配置
COPY config.yaml /app/config.yaml

EXPOSE 8100 8003

USER app
CMD ["/app/jh_gateway"]
