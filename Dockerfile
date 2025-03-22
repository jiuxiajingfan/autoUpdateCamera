# 使用 golang alpine 作为构建环境
FROM golang:1.22-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache gcc musl-dev

# 设置工作目录
WORKDIR /app

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o autoUpdateCam

# 使用 alpine 作为运行环境
FROM alpine:3.19

# 安装 FFmpeg 和必要的 CA 证书
RUN apk add --no-cache ffmpeg ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/autoUpdateCam .

# 创建必要的目录
RUN mkdir -p /app/recordings

# 声明所有环境变量
ENV TZ=Asia/Shanghai \
    CAMERA_IP="" \
    CAMERA_PORT="" \
    CAMERA_USERNAME="" \
    CAMERA_PASSWORD="" \
    CAMERA_STREAM="" \
    RECORDING_OUTPUT_DIR="/app/recordings" \
    RECORDING_SEGMENT_TIME="" \
    RECORDING_START_HOUR="" \
    RECORDING_START_MINUTE="" \
    RECORDING_END_HOUR="" \
    RECORDING_END_MINUTE="" \
    UPLOAD_RETRY_COUNT="" \
    UPLOAD_RETRY_DELAY="" \
    UPLOAD_KEEP_LOCAL="" \
    UPLOAD_FILE_PATTERN="" \
    UPLOAD_MAX_FILE_AGE="" \
    UPLOAD_ALIST_URL="" \
    UPLOAD_ALIST_USER="" \
    UPLOAD_ALIST_PASS="" \
    UPLOAD_ALIST_PATH=""

# 设置启动命令
CMD ["./autoUpdateCam"] 