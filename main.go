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
	file, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
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

	// 在这里进行一次性合并
	if err, mergedFile := r.mergeSegments(); err != nil {
		fmt.Printf("Warning: failed to merge segments: %v\n", err)
	} else if mergedFile != "" {
		// 合并成功后执行上传
		fmt.Printf("Starting to upload merged file: %s\n", mergedFile)
		destPath := filepath.Base(mergedFile) // 使用文件名作为目标路径
		if response, err := r.uploader.UploadFile(mergedFile, destPath); err != nil {
			log.Printf("Warning: failed to upload merged file: %v", err)
		} else {
			// 打印上传响应的JSON
			responseJSON, _ := json.MarshalIndent(response, "", "  ")
			fmt.Printf("Upload response: %s\n", string(responseJSON))
			fmt.Printf("Successfully uploaded file: %s\n", mergedFile)
		}
	}

	// 最后再设置状态为 false
	r.mu.Lock()
	r.isRecording = false
	r.mu.Unlock()
}

func main() {
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
		if !flag {
			println("Waiting for recording period...")
		}
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
