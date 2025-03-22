# Auto Update Camera Recording

这是一个用于自动录制摄像头视频流的程序。它可以根据配置的时间段自动开始和停止录制，并将视频分段保存。

## 功能特点

- 支持 RTSP 视频流录制
- 可配置录制时间段
- 自动分段保存视频
- 支持自动合并视频片段
- 自动重试机制
- 上传录像至天翼云盘

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
    "rtsp_url": "rtsp://admin:admin@192.168.1.100:554/Streaming/channels/101",
    "output_dir": "./recordings",
    "segment_time": 300,
    "start_time": "08:00",
    "end_time": "18:00",
    "retry_count": 3,
    "retry_delay": 5
}
```

配置说明：
- `rtsp_url`: RTSP 视频流地址
- `output_dir`: 视频保存目录
- `segment_time`: 每个视频片段的时长（秒）
- `start_time`: 开始录制时间（格式：HH:MM）
- `end_time`: 结束录制时间（格式：HH:MM）
- `retry_count`: 重试次数
- `retry_delay`: 重试延迟（秒）

## 使用方法

1. 运行程序：
```bash
./autoUpdateCam
```

2. 程序会在配置的时间段内自动开始和停止录制。

3. 录制完成后，视频片段会自动合并。

4. 按 Ctrl+C 可以优雅地停止程序。

## 输出文件

- 视频片段：`segment_XXX.mkv`
- 合并后的视频：`merged_XXXX.mkv`

## 注意事项

1. 确保 FFmpeg 已正确安装并添加到系统 PATH。
2. 确保有足够的磁盘空间存储视频文件。
3. 确保输出目录有写入权限。

## 许可证

MIT License 