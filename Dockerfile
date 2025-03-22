# 使用 golang 官方镜像作为构建环境
FROM golang:1.22-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache gcc musl-dev

# 设置工作目录
WORKDIR /app

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o autoUpdateCam

# 使用 alpine 作为运行环境
FROM alpine:3.19

# 安装 FFmpeg 和其他必要的包
RUN apk add --no-cache ffmpeg ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/autoUpdateCam .

# 创建录制目录
RUN mkdir -p /app/recordings

# 声明所有环境变量
ENV TZ=Asia/Shanghai \
    CAMERA_IP=192.168.1.100 \
    CAMERA_PORT=554 \
    CAMERA_USERNAME=admin \
    CAMERA_PASSWORD=password \
    CAMERA_STREAM="" \
    RECORDING_OUTPUT_DIR="/app/recordings" \
    RECORDING_SEGMENT_TIME=300 \
    RECORDING_START_HOUR=8 \
    RECORDING_START_MINUTE=0 \
    RECORDING_END_HOUR=18 \
    RECORDING_END_MINUTE=0 \
    UPLOAD_RETRY_COUNT=3 \
    UPLOAD_RETRY_DELAY=5 \
    UPLOAD_KEEP_LOCAL=false \
    UPLOAD_FILE_PATTERN=merged_*.mkv \
    UPLOAD_MAX_FILE_AGE=30 \
    UPLOAD_ALIST_URL=http://localhost:5244 \
    UPLOAD_ALIST_USER=admin \
    UPLOAD_ALIST_PASS=password \
    UPLOAD_ALIST_PATH=/ \
    UPLOAD_MAX_CONCURRENT=3

# 设置时区
RUN ln -sf /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone

# 创建启动脚本
RUN printf '#!/bin/sh\n\
./autoUpdateCam \\\n\
  --camera-ip "$CAMERA_IP" \\\n\
  --camera-port "$CAMERA_PORT" \\\n\
  --camera-username "$CAMERA_USERNAME" \\\n\
  --camera-password "$CAMERA_PASSWORD" \\\n\
  --camera-stream "$CAMERA_STREAM" \\\n\
  --recording-output-dir "$RECORDING_OUTPUT_DIR" \\\n\
  --recording-segment-time "$RECORDING_SEGMENT_TIME" \\\n\
  --recording-start-hour "$RECORDING_START_HOUR" \\\n\
  --recording-start-minute "$RECORDING_START_MINUTE" \\\n\
  --recording-end-hour "$RECORDING_END_HOUR" \\\n\
  --recording-end-minute "$RECORDING_END_MINUTE" \\\n\
  --upload-retry-count "$UPLOAD_RETRY_COUNT" \\\n\
  --upload-retry-delay "$UPLOAD_RETRY_DELAY" \\\n\
  --upload-keep-local "$UPLOAD_KEEP_LOCAL" \\\n\
  --upload-file-pattern "$UPLOAD_FILE_PATTERN" \\\n\
  --upload-max-file-age "$UPLOAD_MAX_FILE_AGE" \\\n\
  --upload-alist-url "$UPLOAD_ALIST_URL" \\\n\
  --upload-alist-user "$UPLOAD_ALIST_USER" \\\n\
  --upload-alist-pass "$UPLOAD_ALIST_PASS" \\\n\
  --upload-alist-path "$UPLOAD_ALIST_PATH" \\\n\
  --upload-max-concurrent "$UPLOAD_MAX_CONCURRENT"\n' > /app/start.sh && \
    chmod +x /app/start.sh

# 设置启动命令
CMD ["/bin/sh", "/app/start.sh"]