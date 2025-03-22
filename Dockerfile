# 使用 golang 官方镜像作为构建环境
FROM golang:1.22-bullseye AS builder

# 设置工作目录
WORKDIR /app

# 复制源代码
COPY . .

# 构建应用
RUN go build -o autoUpdateCam

# 使用 debian 作为运行环境
FROM debian:bullseye-slim

# 安装 FFmpeg 和其他必要的包
RUN apt-get update && apt-get install -y \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/autoUpdateCam .

# 创建配置文件目录和录制目录
RUN mkdir -p /app/recordings /app/data

# 复制配置文件
COPY config.json .

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

# 设置时区
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 创建启动脚本
RUN echo '#!/bin/sh\n\
./autoUpdateCam \
  --camera-ip "$CAMERA_IP" \
  --camera-port "$CAMERA_PORT" \
  --camera-username "$CAMERA_USERNAME" \
  --camera-password "$CAMERA_PASSWORD" \
  --camera-stream "$CAMERA_STREAM" \
  --recording-output-dir "$RECORDING_OUTPUT_DIR" \
  --recording-segment-time "$RECORDING_SEGMENT_TIME" \
  --recording-start-hour "$RECORDING_START_HOUR" \
  --recording-start-minute "$RECORDING_START_MINUTE" \
  --recording-end-hour "$RECORDING_END_HOUR" \
  --recording-end-minute "$RECORDING_END_MINUTE" \
  --upload-retry-count "$UPLOAD_RETRY_COUNT" \
  --upload-retry-delay "$UPLOAD_RETRY_DELAY" \
  --upload-keep-local "$UPLOAD_KEEP_LOCAL" \
  --upload-file-pattern "$UPLOAD_FILE_PATTERN" \
  --upload-max-file-age "$UPLOAD_MAX_FILE_AGE" \
  --upload-alist-url "$UPLOAD_ALIST_URL" \
  --upload-alist-user "$UPLOAD_ALIST_USER" \
  --upload-alist-pass "$UPLOAD_ALIST_PASS" \
  --upload-alist-path "$UPLOAD_ALIST_PATH"' > /app/start.sh && \
    chmod +x /app/start.sh

# 设置启动命令
CMD ["/app/start.sh"] 