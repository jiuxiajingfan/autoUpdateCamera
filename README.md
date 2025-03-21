# RTSP Camera Recorder

这是一个使用 Go 语言编写的 RTSP 摄像头录制程序，可以自动将摄像头内容录制为 MKV 格式的视频文件。

## 功能特点

- 支持 RTSP 协议录制
- 每 10 秒生成一个分片文件
- 自动按小时创建文件夹
- 分片文件按顺序命名
- 使用 MKV 格式保存

## 系统要求

- Go 1.16 或更高版本
- FFmpeg（需要预先安装）

## 安装 FFmpeg

### Windows
1. 下载 FFmpeg: https://ffmpeg.org/download.html
2. 将 FFmpeg 添加到系统环境变量

### Linux
```bash
sudo apt update
sudo apt install ffmpeg
```

## 使用方法

1. 修改 `main.go` 中的 RTSP 地址：
```go
rtspURL := "rtsp://your-camera-ip:554/stream" // 替换为你的摄像头 RTSP 地址
```

2. 运行程序：
```bash
go run main.go
```

## 输出文件结构

```
recordings/
└── 20240315_14/              # 日期_小时
    ├── segment_000.mkv       # 10秒分片文件
    ├── segment_001.mkv
    └── ...
```

## 注意事项

- 确保摄像头 RTSP 地址正确且可访问
- 确保有足够的磁盘空间存储视频文件
- 程序会持续运行直到手动停止 