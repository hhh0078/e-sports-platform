# 文件作用：从Go源码构建后端API、演示工具和显式默认运营初始化命令的非root镜像。
FROM m.daocloud.io/docker.io/library/golang:1.24-alpine AS builder
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} CGO_ENABLED=0
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/jijie-api ./cmd/server && go build -trimpath -ldflags="-s -w" -o /out/jijie-demo ./cmd/demo && go build -trimpath -ldflags="-s -w" -o /out/jijie-default-content ./cmd/default-content

FROM m.daocloud.io/docker.io/library/alpine:3.21
ARG SOURCE_COMMIT=unknown
ARG SOURCE_INPUT=unknown
LABEL org.opencontainers.image.revision=$SOURCE_COMMIT cn.58it.jijie.input=$SOURCE_INPUT
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata python3 py3-pip \
    && addgroup -S jijie \
    && adduser -S -G jijie jijie
WORKDIR /app
COPY font-requirements.txt /app/font-requirements.txt
RUN python3 -m venv /opt/font-subset \
    && /opt/font-subset/bin/pip install --no-cache-dir -i https://pypi.tuna.tsinghua.edu.cn/simple -r /app/font-requirements.txt
ENV FONT_SUBSET_PYTHON=/opt/font-subset/bin/python
COPY --from=builder /out/ /app/
USER jijie
EXPOSE 8090
HEALTHCHECK --interval=20s --timeout=4s --start-period=20s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8090/health || exit 1
ENTRYPOINT ["/app/jijie-api"]
