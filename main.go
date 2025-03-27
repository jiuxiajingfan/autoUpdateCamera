package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Camera struct {
		IP       string `json:"ip"`
		Port     string `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Stream   string `json:"stream"`
	} `json:"camera"`
	Recording struct {
		OutputDir   string `json:"output_dir"`
		SegmentTime int    `json:"segment_time"`
		StartHour   int    `json:"start_hour"`
		StartMinute int    `json:"start_minute"`
		EndHour     int    `json:"end_hour"`
		EndMinute   int    `json:"end_minute"`
	} `json:"recording"`
	Upload UploadConfig `json:"upload"`
}

type UploadConfig struct {
	RetryCount    int    `json:"retry_count"`
	RetryDelay    int    `json:"retry_delay"`
	KeepLocal     bool   `json:"keep_local"`
	FilePattern   string `json:"file_pattern"`
	MaxFileAge    int    `json:"max_file_age"`
	AlistURL      string `json:"alist_url"`
	AlistUser     string `json:"alist_user"`
	AlistPass     string `json:"alist_pass"`
	AlistPath     string `json:"alist_path"`
	MaxConcurrent int    `json:"max_concurrent"`
}

type Recorder struct {
	rtspURL     string
	outputDir   string
	segmentTime int
	stopChan    chan struct{}
	sequence    int
	currentCmd  *exec.Cmd
	isWindows   bool
	startTime   time.Time
	endTime     time.Time
	retryCount  int
	isRecording bool
	startChan   chan struct{}
	mu          sync.Mutex // 添加互斥锁
	uploader    *FileUploader
}

func loadConfig() (*Config, error) {
	config := &Config{}

	// 从环境变量加载摄像头配置
	config.Camera.IP = getEnvOrDefault("CAMERA_IP", "192.168.1.100")
	config.Camera.Port = getEnvOrDefault("CAMERA_PORT", "554")
	config.Camera.Username = getEnvOrDefault("CAMERA_USERNAME", "admin")
	config.Camera.Password = getEnvOrDefault("CAMERA_PASSWORD", "password")
	config.Camera.Stream = getEnvOrDefault("CAMERA_STREAM", "/cam/realmonitor?channel=1&subtype=0")

	// 从环境变量加载录制配置
	config.Recording.OutputDir = getEnvOrDefault("RECORDING_OUTPUT_DIR", "recordings")
	config.Recording.SegmentTime = getEnvIntOrDefault("RECORDING_SEGMENT_TIME", 300)
	config.Recording.StartHour = getEnvIntOrDefault("RECORDING_START_HOUR", 8)
	config.Recording.StartMinute = getEnvIntOrDefault("RECORDING_START_MINUTE", 0)
	config.Recording.EndHour = getEnvIntOrDefault("RECORDING_END_HOUR", 18)
	config.Recording.EndMinute = getEnvIntOrDefault("RECORDING_END_MINUTE", 0)

	// 从环境变量加载上传配置
	config.Upload.RetryCount = getEnvIntOrDefault("UPLOAD_RETRY_COUNT", 3)
	config.Upload.RetryDelay = getEnvIntOrDefault("UPLOAD_RETRY_DELAY", 5)
	config.Upload.KeepLocal = getEnvBoolOrDefault("UPLOAD_KEEP_LOCAL", true)
	config.Upload.FilePattern = getEnvOrDefault("UPLOAD_FILE_PATTERN", "merged_*.mkv")
	config.Upload.MaxFileAge = getEnvIntOrDefault("UPLOAD_MAX_FILE_AGE", 30)
	config.Upload.AlistURL = getEnvOrDefault("UPLOAD_ALIST_URL", "http://localhost:5244")
	config.Upload.AlistUser = getEnvOrDefault("UPLOAD_ALIST_USER", "admin")
	config.Upload.AlistPass = getEnvOrDefault("UPLOAD_ALIST_PASS", "password")
	config.Upload.AlistPath = getEnvOrDefault("UPLOAD_ALIST_PATH", "/")
	config.Upload.MaxConcurrent = getEnvIntOrDefault("UPLOAD_MAX_CONCURRENT", 3)

	// 打印实际使用的配置
	log.Printf("Using configuration:")
	log.Printf("Camera: IP=%s, Port=%s, Username=%s, Stream=%s",
		config.Camera.IP, config.Camera.Port, config.Camera.Username, config.Camera.Stream)
	log.Printf("Recording: OutputDir=%s, SegmentTime=%d, Start=%02d:%02d, End=%02d:%02d",
		config.Recording.OutputDir, config.Recording.SegmentTime,
		config.Recording.StartHour, config.Recording.StartMinute,
		config.Recording.EndHour, config.Recording.EndMinute)
	log.Printf("Upload: RetryCount=%d, RetryDelay=%d, KeepLocal=%v, FilePattern=%s, MaxFileAge=%d",
		config.Upload.RetryCount, config.Upload.RetryDelay, config.Upload.KeepLocal,
		config.Upload.FilePattern, config.Upload.MaxFileAge)
	log.Printf("Alist: URL=%s, User=%s, Path=%s",
		config.Upload.AlistURL, config.Upload.AlistUser, config.Upload.AlistPath)

	// 尝试从文件加载配置（如果存在）
	if _, err := os.Stat("config.json"); err == nil {
		file, err := os.ReadFile("config.json")
		if err == nil {
			// 解析 JSON 配置
			var fileConfig Config
			if err := json.Unmarshal(file, &fileConfig); err == nil {
				// 使用文件配置覆盖默认值和环境变量（如果文件中有相应配置）
				mergeConfig(config, &fileConfig)
			}
		}
	}

	return config, nil
}

// 从环境变量获取字符串值，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// 从环境变量获取整数值，如果不存在或无效则返回默认值
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// 从环境变量获取布尔值，如果不存在或无效则返回默认值
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// 合并配置，用源配置中的非零值覆盖目标配置
func mergeConfig(dst, src *Config) {
	// 合并摄像头配置
	if src.Camera.IP != "" {
		dst.Camera.IP = src.Camera.IP
	}
	if src.Camera.Port != "" {
		dst.Camera.Port = src.Camera.Port
	}
	if src.Camera.Username != "" {
		dst.Camera.Username = src.Camera.Username
	}
	if src.Camera.Password != "" {
		dst.Camera.Password = src.Camera.Password
	}
	if src.Camera.Stream != "" {
		dst.Camera.Stream = src.Camera.Stream
	}

	// 合并录制配置
	if src.Recording.OutputDir != "" {
		dst.Recording.OutputDir = src.Recording.OutputDir
	}
	if src.Recording.SegmentTime != 0 {
		dst.Recording.SegmentTime = src.Recording.SegmentTime
	}
	if src.Recording.StartHour != 0 {
		dst.Recording.StartHour = src.Recording.StartHour
	}
	if src.Recording.StartMinute != 0 {
		dst.Recording.StartMinute = src.Recording.StartMinute
	}
	if src.Recording.EndHour != 0 {
		dst.Recording.EndHour = src.Recording.EndHour
	}
	if src.Recording.EndMinute != 0 {
		dst.Recording.EndMinute = src.Recording.EndMinute
	}

	// 合并上传配置
	if src.Upload.RetryCount != 0 {
		dst.Upload.RetryCount = src.Upload.RetryCount
	}
	if src.Upload.RetryDelay != 0 {
		dst.Upload.RetryDelay = src.Upload.RetryDelay
	}
	if src.Upload.FilePattern != "" {
		dst.Upload.FilePattern = src.Upload.FilePattern
	}
	if src.Upload.MaxFileAge != 0 {
		dst.Upload.MaxFileAge = src.Upload.MaxFileAge
	}
	if src.Upload.AlistURL != "" {
		dst.Upload.AlistURL = src.Upload.AlistURL
	}
	if src.Upload.AlistUser != "" {
		dst.Upload.AlistUser = src.Upload.AlistUser
	}
	if src.Upload.AlistPass != "" {
		dst.Upload.AlistPass = src.Upload.AlistPass
	}
	if src.Upload.AlistPath != "" {
		dst.Upload.AlistPath = src.Upload.AlistPath
	}
	if src.Upload.MaxConcurrent != 0 {
		dst.Upload.MaxConcurrent = src.Upload.MaxConcurrent
	}
}

func NewRecorder(config *Config, startTime, endTime time.Time) *Recorder {
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:%s/%s",
		config.Camera.Username,
		config.Camera.Password,
		config.Camera.IP,
		config.Camera.Port,
		config.Camera.Stream)

	return &Recorder{
		rtspURL:     rtspURL,
		outputDir:   config.Recording.OutputDir,
		segmentTime: config.Recording.SegmentTime,
		stopChan:    make(chan struct{}),
		startChan:   make(chan struct{}),
		sequence:    0,
		isWindows:   runtime.GOOS == "windows",
		startTime:   startTime,
		endTime:     endTime,
		retryCount:  0,
		isRecording: false,
		uploader:    NewFileUploader(&config.Upload),
	}
}

func (r *Recorder) startFFmpeg() error {
	absOutputDir, err := filepath.Abs(r.outputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	outputPattern := filepath.Join(absOutputDir, "segment_%03d.mkv")
	if r.isWindows {
		outputPattern = strings.ReplaceAll(outputPattern, "\\", "/")
	}

	args := []string{
		"-rtsp_transport", "tcp",
		"-timeout", "5000000", // 设置超时时间为5秒
		"-i", r.rtspURL,
		"-c", "copy",
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", r.segmentTime),
		"-segment_format", "matroska",
		"-reset_timestamps", "1",
		"-fflags", "+genpts",
		outputPattern,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = absOutputDir

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start recording: %v", err)
	}

	r.currentCmd = cmd
	return nil
}

func (r *Recorder) mergeSegments() (error, string) {
	absOutputDir, err := filepath.Abs(r.outputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err), ""
	}

	time.Sleep(5 * time.Second)

	// 强制结束所有 ffmpeg 进程
	if r.isWindows {
		exec.Command("taskkill", "/F", "/IM", "ffmpeg.exe").Run()
	} else {
		exec.Command("pkill", "-9", "ffmpeg").Run()
	}
	time.Sleep(5 * time.Second)

	files, err := os.ReadDir(absOutputDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err), ""
	}

	var validSegments []string
	var invalidSegments []string
	segmentCount := 0

	// 首先统计所有片段文件
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "segment_") && strings.HasSuffix(file.Name(), ".mkv") {
			segmentCount++
		}
	}

	fmt.Printf("Found %d total segment files\n", segmentCount)

	// 收集有效和无效的片段
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "segment_") && strings.HasSuffix(file.Name(), ".mkv") {
			filePath := filepath.Join(absOutputDir, file.Name())
			info, err := os.Stat(filePath)
			if err != nil || info.Size() < 1024 {
				invalidSegments = append(invalidSegments, file.Name())
				// 删除无效的分片文件
				if err := os.Remove(filePath); err != nil {
					log.Printf("Warning: failed to remove invalid segment file %s: %v", filePath, err)
				}
				continue
			}
			validSegments = append(validSegments, file.Name())
		}
	}

	fmt.Printf("Found %d valid segments and %d invalid segments\n", len(validSegments), len(invalidSegments))

	if len(validSegments) == 0 {
		return fmt.Errorf("no valid segments found to merge"), ""
	}

	// 对片段进行数字排序
	sort.Slice(validSegments, func(i, j int) bool {
		// 从文件名中提取数字
		numI := strings.TrimPrefix(strings.TrimSuffix(validSegments[i], ".mkv"), "segment_")
		numJ := strings.TrimPrefix(strings.TrimSuffix(validSegments[j], ".mkv"), "segment_")
		// 将数字字符串转换为整数进行比较
		iNum, _ := strconv.Atoi(numI)
		jNum, _ := strconv.Atoi(numJ)
		return iNum < jNum
	})

	// 创建合并列表文件
	listFile := filepath.Join(absOutputDir, "concat_list.txt")
	content := ""
	for _, segment := range validSegments {
		segmentPath := filepath.Join(absOutputDir, segment)
		// 再次验证文件是否存在
		if _, err := os.Stat(segmentPath); err != nil {
			log.Printf("Warning: segment file %s no longer exists, skipping", segmentPath)
			continue
		}
		if r.isWindows {
			content += fmt.Sprintf("file '%s'\n", strings.ReplaceAll(segment, "\\", "/"))
		} else {
			content += fmt.Sprintf("file '%s'\n", segment)
		}
	}

	fmt.Printf("Writing concat list with %d segments\n", len(validSegments))

	if content == "" {
		return fmt.Errorf("no valid segments available for merging"), ""
	}

	if err := os.WriteFile(listFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create concat list: %v", err), ""
	}

	// 输出concat_list.txt的内容以供验证
	fmt.Println("Contents of concat_list.txt:")
	fmt.Println(content)

	// 设置输出文件路径
	now := time.Now()
	dateStr := now.Format("20060102") // 格式化日期为 YYYYMMDD
	outputFile := filepath.Join(absOutputDir, fmt.Sprintf("merged_%s.mkv", dateStr))
	r.sequence++

	// 尝试合并，最多重试3次
	maxRetries := 3
	var mergeSuccess bool
	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("Attempting to merge segments (attempt %d/%d)...\n", attempt, maxRetries)

		args := []string{
			"-f", "concat",
			"-safe", "0",
			"-i", listFile,
			"-c", "copy",
			outputFile,
		}

		cmd := exec.Command("ffmpeg", args...)
		cmd.Dir = absOutputDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Merge attempt %d failed: %v\n", attempt, err)
			if attempt < maxRetries {
				fmt.Println("Waiting 5 seconds before retry...")
				time.Sleep(5 * time.Second)
				continue
			}
			// 删除可能存在的不完整输出文件
			os.Remove(outputFile)
			return fmt.Errorf("all merge attempts failed: %v"), ""
		}

		// 验证输出文件
		if info, err := os.Stat(outputFile); err != nil || info.Size() < 1024 {
			fmt.Printf("Output file verification failed: %v\n", err)
			if attempt < maxRetries {
				fmt.Println("Waiting 5 seconds before retry...")
				time.Sleep(5 * time.Second)
				continue
			}
			// 删除无效的输出文件
			os.Remove(outputFile)
			return fmt.Errorf("output file verification failed after all attempts"), ""
		}

		mergeSuccess = true
		break
	}

	if !mergeSuccess {
		return fmt.Errorf("merge failed after %d attempts", maxRetries), ""
	}

	// 合并成功后，删除原始分片文件
	fmt.Println("Merge successful, cleaning up segment files...")
	for _, segment := range validSegments {
		segmentPath := filepath.Join(absOutputDir, segment)
		// 先检查文件是否存在
		if _, err := os.Stat(segmentPath); err != nil {
			continue // 文件不存在，跳过
		}

		for i := 0; i < 10; i++ {
			if r.isWindows {
				exec.Command("taskkill", "/F", "/IM", "ffmpeg.exe").Run()
			} else {
				exec.Command("pkill", "-9", "ffmpeg").Run()
			}
			time.Sleep(1 * time.Second)

			err := os.Remove(segmentPath)
			if err == nil {
				break
			}
			if err != nil && strings.Contains(err.Error(), "being used by another process") {
				time.Sleep(2 * time.Second)
				continue
			}
			log.Printf("Warning: failed to remove segment file %s: %v", segmentPath, err)
			break
		}
	}

	// 删除 concat_list.txt 文件
	if err := os.Remove(listFile); err != nil {
		log.Printf("Warning: failed to remove concat list file: %v", err)
	} else {
		fmt.Printf("Successfully removed concat list file: %s\n", listFile)
	}

	fmt.Printf("Successfully merged %d segments into %s\n", len(validSegments), outputFile)

	return nil, outputFile
}

func (r *Recorder) stopFFmpeg() error {
	time.Sleep(5 * time.Second)
	if r.currentCmd != nil && r.currentCmd.Process != nil {
		done := make(chan error)
		go func() {
			done <- r.currentCmd.Wait()
		}()

		if r.isWindows {
			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", r.currentCmd.Process.Pid)).Run()
		} else {
			exec.Command("kill", "-9", fmt.Sprintf("%d", r.currentCmd.Process.Pid)).Run()
		}

		select {
		case err := <-done:
			if err != nil && !strings.Contains(err.Error(), "signal: killed") {
				return fmt.Errorf("process exited with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timeout waiting for process to exit")
		}

		r.currentCmd = nil
	}
	return nil
}

func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRecording
}

func (r *Recorder) Start() {
	r.mu.Lock()
	if r.isRecording {
		r.mu.Unlock()
		return
	}
	r.isRecording = true
	r.mu.Unlock()
	close(r.startChan)
}

func (r *Recorder) StartRecording() error {
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 等待开始信号
	<-r.startChan

	for {
		now := time.Now()
		if now.After(r.endTime) {
			fmt.Printf("Reached end time %s, stopping recording...\n", r.endTime.Format("15:04:05"))
			if r.currentCmd != nil && r.currentCmd.Process != nil {
				if err := r.stopFFmpeg(); err != nil {
					fmt.Printf("Warning: failed to stop ffmpeg process: %v\n", err)
				}
			}
			time.Sleep(5 * time.Second)

			// 不在这里合并片段
			r.mu.Lock()
			r.isRecording = false
			r.mu.Unlock()
			return nil
		}

		select {
		case <-r.stopChan:
			if r.currentCmd != nil && r.currentCmd.Process != nil {
				if err := r.stopFFmpeg(); err != nil {
					fmt.Printf("Warning: failed to stop ffmpeg process: %v\n", err)
				}
			}
			time.Sleep(5 * time.Second)

			// 不在这里合并片段
			r.mu.Lock()
			r.isRecording = false
			r.mu.Unlock()
			return nil
		default:
			if err := r.startFFmpeg(); err != nil {
				r.retryCount++
				fmt.Printf("Error starting ffmpeg (attempt %d): %v\n", r.retryCount, err)
				time.Sleep(5 * time.Second)
				continue
			}

			// 重置重试计数
			r.retryCount = 0
			fmt.Println("Successfully connected to camera")

			if err := r.currentCmd.Wait(); err != nil {
				r.retryCount++
				fmt.Printf("Warning: ffmpeg process exited with error (attempt %d): %v\n", r.retryCount, err)
				time.Sleep(5 * time.Second)
				continue
			}

			time.Sleep(5 * time.Second)
		}
	}
}

func (r *Recorder) Stop() {
	r.mu.Lock()
	if !r.isRecording {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	close(r.stopChan)
	// 等待录制完全停止
	if r.currentCmd != nil && r.currentCmd.Process != nil {
		if err := r.stopFFmpeg(); err != nil {
			fmt.Printf("Warning: failed to stop ffmpeg process: %v\n", err)
		}
	}
	time.Sleep(5 * time.Second)

	// 获取录制结束时的日期
	recordingEndDate := time.Now().Format("20060102")
	fmt.Printf("Recording ended at %s, using this date for all uploads\n", recordingEndDate)

	// 在新的 goroutine 中处理上传
	go func() {
		// 获取录制目录的绝对路径
		absOutputDir, err := filepath.Abs(r.outputDir)
		if err != nil {
			fmt.Printf("Error getting absolute path: %v\n", err)
			return
		}

		// 获取所有分段文件
		files, err := os.ReadDir(absOutputDir)
		if err != nil {
			fmt.Printf("Error reading directory: %v\n", err)
			return
		}

		var validSegments []string
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "segment_") && strings.HasSuffix(file.Name(), ".mkv") {
				filePath := filepath.Join(absOutputDir, file.Name())
				info, err := os.Stat(filePath)
				if err != nil || info.Size() < 1024 {
					// 删除无效的分片文件
					if err := os.Remove(filePath); err != nil {
						log.Printf("Warning: failed to remove invalid segment file %s: %v", filePath, err)
					}
					continue
				}
				validSegments = append(validSegments, file.Name())
			}
		}

		// 按文件名排序
		sort.Slice(validSegments, func(i, j int) bool {
			numI := strings.TrimPrefix(strings.TrimSuffix(validSegments[i], ".mkv"), "segment_")
			numJ := strings.TrimPrefix(strings.TrimSuffix(validSegments[j], ".mkv"), "segment_")
			iNum, _ := strconv.Atoi(numI)
			jNum, _ := strconv.Atoi(numJ)
			return iNum < jNum
		})

		fmt.Printf("Found %d valid segments to upload\n", len(validSegments))

		if len(validSegments) == 0 {
			fmt.Println("No valid segments to upload")
			return
		}

		// 创建任务通道和等待组
		tasks := make(chan string, len(validSegments))
		var wg sync.WaitGroup

		// 创建上传状态管理
		type uploadStatus struct {
			sync.Mutex
			inProgress map[string]bool
			completed  map[string]bool
		}
		status := &uploadStatus{
			inProgress: make(map[string]bool),
			completed:  make(map[string]bool),
		}

		// 启动工作协程
		maxWorkers := r.uploader.config.MaxConcurrent
		if maxWorkers <= 0 {
			maxWorkers = 3 // 默认值
		}

		fmt.Printf("Starting %d upload workers\n", maxWorkers)

		// 创建工作协程
		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for segment := range tasks {
					// 检查文件是否已经在上传或已完成
					status.Lock()
					if status.inProgress[segment] || status.completed[segment] {
						status.Unlock()
						continue
					}
					status.inProgress[segment] = true
					status.Unlock()

					segmentPath := filepath.Join(absOutputDir, segment)
					destPath := filepath.Join(r.uploader.config.AlistPath, recordingEndDate, segment)

					fmt.Printf("[Worker %d] Uploading segment: %s to %s\n", workerID, segment, destPath)

					// 尝试上传文件
					var uploadErr error
					var uploadSuccess bool
					for i := 0; i < r.uploader.config.RetryCount; i++ {
						if response, err := r.uploader.UploadFile(segmentPath, destPath, recordingEndDate); err != nil {
							uploadErr = err
							log.Printf("[Worker %d] Upload attempt %d/%d failed for %s: %v",
								workerID, i+1, r.uploader.config.RetryCount, segment, err)
							time.Sleep(time.Duration(r.uploader.config.RetryDelay) * time.Second)
							continue
						} else {
							responseJSON, _ := json.MarshalIndent(response, "", "  ")
							fmt.Printf("[Worker %d] Upload response for %s: %s\n", workerID, segment, string(responseJSON))
							uploadErr = nil
							uploadSuccess = true
							break
						}
					}

					// 更新上传状态
					status.Lock()
					delete(status.inProgress, segment)
					if uploadSuccess {
						status.completed[segment] = true
					}
					status.Unlock()

					if uploadErr != nil {
						log.Printf("[Worker %d] Failed to upload segment %s after %d attempts: %v",
							workerID, segment, r.uploader.config.RetryCount, uploadErr)
					}
				}
				fmt.Printf("[Worker %d] Finished processing all assigned segments\n", workerID)
			}(i)
		}

		// 发送任务到通道
		fmt.Printf("Queueing %d segments for upload\n", len(validSegments))
		for _, segment := range validSegments {
			tasks <- segment
		}
		close(tasks)

		// 等待所有上传完成
		fmt.Println("Waiting for all uploads to complete...")
		wg.Wait()

		// 打印上传统计
		status.Lock()
		fmt.Printf("Upload summary: %d/%d files successfully uploaded\n",
			len(status.completed), len(validSegments))
		status.Unlock()

		fmt.Println("All uploads completed")
	}()

	// 立即设置状态为 false，不等待上传完成
	r.mu.Lock()
	r.isRecording = false
	r.mu.Unlock()
}

func main() {
	fmt.Println("Version: 0.1")
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}
	now := time.Now()
	startTime := time.Date(now.Year(), now.Month(), now.Day(), config.Recording.StartHour, config.Recording.StartMinute, 0, 0, now.Location())
	endTime := time.Date(now.Year(), now.Month(), now.Day(), config.Recording.EndHour, config.Recording.EndMinute, 0, 0, now.Location())

	// 创建一个全局的录制器实例
	recorder := NewRecorder(config, startTime, endTime)
	recordingDone := make(chan struct{})

	// 启动录制逻辑的 goroutine
	go func() {
		if err := recorder.StartRecording(); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		close(recordingDone)
	}()
	println("start success! Waiting for recording period...")
	flag := false
	for {
		now := time.Now()
		if now.After(startTime) && now.Before(endTime) {
			// 开始逻辑：如果未在录制，则开始录制
			if !recorder.IsRecording() && !flag {
				flag = true
				fmt.Printf("Current time %s is within recording period, starting recording...\n", now.Format("15:04:05"))
				recorder.Start()
			}
		} else if now.After(endTime) {
			// 终止逻辑：如果正在录制，则停止录制
			if recorder.IsRecording() && flag {
				flag = false
				fmt.Printf("Reached end time %s, stopping recording...\n", endTime.Format("15:04:05"))
				recorder.Stop()
				println("Waiting for recording period...")
				<-recordingDone // 等待录制完全停止
				// 重置开始和结束时间到下一天
				startTime = startTime.Add(24 * time.Hour)
				endTime = endTime.Add(24 * time.Hour)
			}
		}
		time.Sleep(1 * time.Second)
	}
}
