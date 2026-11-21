# 运行镜像：单个已内嵌 Mellow 前端的二进制（GoReleaser dockers_v2 构建时使用）
#
# ⚠ 构建上下文布局：dockers_v2 是多平台一次构建，会把二进制按平台分目录放好：
#     <os>/<arch>/inis      例如 linux/amd64/inis、linux/arm64/inis
#   所以 COPY 必须带上 ${TARGETOS}/${TARGETARCH}，不能直接写 COPY inis（会报 "/inis: not found"）。
#   TARGETOS / TARGETARCH 是 BuildKit 的预定义参数，跨平台构建时自动注入。
#
# 手动构建（本地验证镜像用，沿用同样的目录布局）：
#   mkdir -p linux/amd64 && cp inis linux/amd64/inis
#   docker build -t inis:local .
FROM alpine:3.21

ARG TARGETOS=linux
ARG TARGETARCH=amd64

# ca-certificates：HTTPS 调用（短信 / 对象存储 / OAuth / 邮件）
# tzdata：时区（容器默认 TZ=Asia/Shanghai，可用 -e TZ= 覆盖）
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S inis \
    && adduser -S -G inis -h /app inis

WORKDIR /app

# 二进制已在构建机完成 strip + UPX，这里只补权限与数据目录
COPY ${TARGETOS}/${TARGETARCH}/inis /usr/local/bin/inis
RUN chmod +x /usr/local/bin/inis \
    && mkdir -p /app/config /app/runtime /app/storage \
    && chown -R inis:inis /app

# 非 root 运行：程序只写 /app 下的 config / runtime / storage
USER inis

ENV TZ=Asia/Shanghai
EXPOSE 8642

# 首次启动会在 /app 下生成 config：数据库、短信、存储密钥都在这里，务必挂载持久化
VOLUME ["/app/config", "/app/runtime", "/app/storage"]

ENTRYPOINT ["/usr/local/bin/inis"]
