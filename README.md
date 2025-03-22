# Auto Update Camera Recording

这是一个用于自动录制摄像头视频流的程序。它可以根据配置的时间段自动开始和停止录制，并将视频分段保存，合并后自动上传至 Alist。

## 功能特点

- 支持 RTSP 视频流录制
- 可配置录制时间段
- 自动分段保存视频
- 自动合并视频片段
- 自动重试机制
- 支持上传至 Alist
- 自动压缩文件（当压缩有效时）
- 自动清理过期文件
- 支持 Docker 部署
- 支持环境变量配置

## 系统要求

### 直接运行
- Go 1.16 或更高版本
- FFmpeg 已安装并添加到系统 PATH

### Docker 部署
- Docker
- Docker Compose

## 安装

### 方式一：直接运行

1. 克隆仓库：
```bash
git clone https://github.com/yourusername/autoUpdateCam.git
cd autoUpdateCam
```

2. 编译程序：
```bash
go build
```

### 方式二：Docker 部署

1. 克隆仓库：
```bash
git clone https://github.com/yourusername/autoUpdateCam.git
cd autoUpdateCam
```

2. 创建并编辑环境配置文件：
```bash
cp .env.example .env
# 使用编辑器修改 .env 文件中的配置
```

3. 构建并启动容器：
```bash
docker-compose up -d
```

4. 查看日志：
```bash
docker-compose logs -f
```

5. 停止服务：
```bash
docker-compose down
```

## 配置

### 方式一：配置文件

编辑 `config.json` 文件来配置程序：

```json
{
    "camera": {
        "ip": "192.168.1.100",
        "port": "554",
        "username": "admin",
        "password": "password",
        "stream": "/cam/realmonitor?channel=1&subtype=0"
    },
    "recording": {
        "output_dir": "recordings",
        "segment_time": 300,
        "start_hour": 8,
        "start_minute": 0,
        "end_hour": 18,
        "end_minute": 0
    },
    "upload": {
        "retry_count": 3,
        "retry_delay": 5,
        "keep_local": true,
        "file_pattern": "merged_*.mkv",
        "max_file_age": 30,
        "alist_url": "http://your-alist-server:5244",
        "alist_user": "admin",
        "alist_pass": "password",
        "alist_path": "/your/upload/path"
    }
}
```

### 方式二：环境变量

使用环境变量配置程序（推荐用于 Docker 部署）：

```env
# 时区设置
TZ=Asia/Shanghai

# 摄像头配置
CAMERA_IP=192.168.1.100
CAMERA_PORT=554
CAMERA_USERNAME=admin
CAMERA_PASSWORD=password
CAMERA_STREAM=/cam/realmonitor?channel=1&subtype=0

# 录制配置
RECORDING_OUTPUT_DIR=/app/recordings
RECORDING_SEGMENT_TIME=300
RECORDING_START_HOUR=8
RECORDING_START_MINUTE=0
RECORDING_END_HOUR=18
RECORDING_END_MINUTE=0

# 上传配置
UPLOAD_RETRY_COUNT=3
UPLOAD_RETRY_DELAY=5
UPLOAD_KEEP_LOCAL=true
UPLOAD_FILE_PATTERN=merged_*.mkv
UPLOAD_MAX_FILE_AGE=30
UPLOAD_ALIST_URL=http://your-alist-server:5244
UPLOAD_ALIST_USER=admin
UPLOAD_ALIST_PASS=password
UPLOAD_ALIST_PATH=/your/upload/path
```

配置说明：

### 摄像头配置
- `CAMERA_IP`: 摄像头 IP 地址
- `CAMERA_PORT`: RTSP 端口
- `CAMERA_USERNAME`: 摄像头用户名
- `CAMERA_PASSWORD`: 摄像头密码
- `CAMERA_STREAM`: RTSP 流路径

### 录制配置
- `RECORDING_OUTPUT_DIR`: 视频保存目录
- `RECORDING_SEGMENT_TIME`: 每个视频片段的时长（秒）
- `RECORDING_START_HOUR`: 开始录制的小时（24小时制）
- `RECORDING_START_MINUTE`: 开始录制的分钟
- `RECORDING_END_HOUR`: 结束录制的小时（24小时制）
- `RECORDING_END_MINUTE`: 结束录制的分钟

### 上传配置
- `UPLOAD_RETRY_COUNT`: 上传失败重试次数
- `UPLOAD_RETRY_DELAY`: 重试间隔（秒）
- `UPLOAD_KEEP_LOCAL`: 是否保留本地文件
- `UPLOAD_FILE_PATTERN`: 要上传的文件匹配模式
- `UPLOAD_MAX_FILE_AGE`: 文件最大保留天数
- `UPLOAD_ALIST_URL`: Alist 服务器地址
- `UPLOAD_ALIST_USER`: Alist 用户名
- `UPLOAD_ALIST_PASS`: Alist 密码
- `UPLOAD_ALIST_PATH`: Alist 上传目录路径

## 使用方法

### 直接运行

1. 确保配置文件正确设置。

2. 运行程序：
```bash
./autoUpdateCam
```

### Docker 运行

1. 确保 `.env` 文件正确配置。

2. 启动容器：
```bash
docker-compose up -d
```

3. 程序会：
   - 在配置的时间段内自动开始和停止录制
   - 将视频分段保存
   - 在录制结束后自动合并视频片段
   - 尝试压缩合并后的文件（如果压缩有效）
   - 上传到 Alist 服务器
   - 根据配置清理本地文件

## 输出文件

- 视频片段：`segment_XXX.mkv`
- 合并后的视频：`merged_YYYYMMDD.mkv`
- 压缩后的文件：`merged_YYYYMMDD.zip`（仅当压缩有效时）

## 注意事项

1. 确保 FFmpeg 已正确安装并添加到系统 PATH（直接运行时）。
2. 确保有足够的磁盘空间存储视频文件。
3. 确保输出目录有写入权限。
4. 确保 Alist 服务器地址可访问且配置正确。
5. 建议定期检查日志确保录制和上传正常。

### Docker 部署注意事项

1. 确保 Docker 和 Docker Compose 已正确安装。
2. 确保 `.env` 文件中的配置正确。
3. 容器会自动重启，无需额外的守护进程。
4. 可以通过 `docker-compose logs -f` 查看实时日志。
5. 录制的文件会保存在宿主机的 `./recordings` 目录中。
6. 可以通过 `./data` 目录访问和管理文件。

## 目录说明

- `./recordings`: 主要的录制文件存储目录，程序默认的输出目录
- `./data`: 用户自定义的访问目录，可以用于文件管理和查看

## 许可证

MIT License 