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

## 系统要求

- Go 1.16 或更高版本
- FFmpeg 已安装并添加到系统 PATH

## 安装

1. 克隆仓库：
```bash
git clone https://github.com/yourusername/autoUpdateCam.git
cd autoUpdateCam
```

2. 编译程序：
```bash
go build
```

## 配置

编辑 `config.json` 文件来配置程序：

```json
{
    "camera": {
        "ip": "192.168.1.100",
        "port": "554",
        "username": "admin",
        "password": "password",
//      各品牌不同，需要根据实际情况修改
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

配置说明：

### 摄像头配置 (camera)
- `ip`: 摄像头 IP 地址
- `port`: RTSP 端口
- `username`: 摄像头用户名
- `password`: 摄像头密码
- `stream`: RTSP 流路径

### 录制配置 (recording)
- `output_dir`: 视频保存目录
- `segment_time`: 每个视频片段的时长（秒）
- `start_hour`: 开始录制的小时（24小时制）
- `start_minute`: 开始录制的分钟
- `end_hour`: 结束录制的小时（24小时制）
- `end_minute`: 结束录制的分钟

### 上传配置 (upload)
- `retry_count`: 上传失败重试次数
- `retry_delay`: 重试间隔（秒）
- `keep_local`: 是否保留本地文件
- `file_pattern`: 要上传的文件匹配模式
- `max_file_age`: 文件最大保留天数
- `alist_url`: Alist 服务器地址
- `alist_user`: Alist 用户名
- `alist_pass`: Alist 密码
- `alist_path`: Alist 上传目录路径

## 使用方法

1. 确保配置文件正确设置。

2. 运行程序：
```bash
./autoUpdateCam
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

1. 确保 FFmpeg 已正确安装并添加到系统 PATH。
2. 确保有足够的磁盘空间存储视频文件。
3. 确保输出目录有写入权限。
4. 确保 Alist 服务器地址可访问且配置正确。
5. 建议定期检查日志确保录制和上传正常。

## 许可证

MIT License 