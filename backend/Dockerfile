# 文件作用：从Go源码构建后端API、演示工具和显式默认运营初始化命令的非root镜像。
FROM golang:1.24-alpine AS builder
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} CGO_ENABLED=0
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/jijie-api ./cmd/server && go build -trimpath -ldflags="-s -w" -o /out/jijie-demo ./cmd/demo && go build -trimpath -ldflags="-s -w" -o /out/jijie-default-content ./cmd/default-content

FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S jijie \
    && adduser -S -G jijie jijie
WORKDIR /app
COPY --from=builder /out/ /app/
USER jijie
EXPOSE 8090
HEALTHCHECK --interval=20s --timeout=4s --start-period=20s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8090/health || exit 1
ENTRYPOINT ["/app/jijie-api"]
