package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileUploader 文件上传器
type FileUploader struct {
	config *UploadConfig
	token  string
}

// NewFileUploader 创建新的文件上传器
func NewFileUploader(config *UploadConfig) *FileUploader {
	return &FileUploader{
		config: config,
	}
}

// getAlistToken 获取Alist token
func (u *FileUploader) getAlistToken() error {
	// 准备登录请求数据
	loginData := map[string]string{
		"username": u.config.AlistUser,
		"password": u.config.AlistPass,
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("failed to marshal login data: %v", err)
	}

	// 创建登录请求
	req, err := http.NewRequest("POST", u.config.AlistURL+"/api/auth/login", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create login request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send login request: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode login response: %v", err)
	}

	// 检查响应状态
	if result.Code != 200 {
		return fmt.Errorf("login failed: %s", result.Message)
	}

	// 保存token
	u.token = result.Data.Token
	log.Printf("Successfully obtained Alist token")
	return nil
}

// compressToZip 将文件压缩为zip格式，如果压缩效果不理想则返回原文件
func (u *FileUploader) compressToZip(inputFile string) (string, bool, error) {
	// 获取原始文件大小
	originalInfo, err := os.Stat(inputFile)
	if err != nil {
		return "", false, fmt.Errorf("failed to get original file info: %v", err)
	}
	originalSize := originalInfo.Size()

	// 创建压缩后的文件路径
	dir := filepath.Dir(inputFile)
	filename := filepath.Base(inputFile)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	zipFile := filepath.Join(dir, nameWithoutExt+".zip")

	// 创建zip文件
	zipWriter, err := os.Create(zipFile)
	if err != nil {
		return "", false, fmt.Errorf("failed to create zip file: %v", err)
	}
	defer zipWriter.Close()

	// 创建zip writer
	archive := zip.NewWriter(zipWriter)
	defer archive.Close()

	// 打开源文件
	file, err := os.Open(inputFile)
	if err != nil {
		return "", false, fmt.Errorf("failed to open source file: %v", err)
	}
	defer file.Close()

	// 创建zip文件头
	header, err := zip.FileInfoHeader(originalInfo)
	if err != nil {
		return "", false, fmt.Errorf("failed to create zip header: %v", err)
	}
	header.Method = zip.Deflate // 使用压缩
	header.Name = filename      // 设置zip中的文件名

	// 创建writer
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return "", false, fmt.Errorf("failed to create zip writer: %v", err)
	}

	// 复制文件内容到zip
	if _, err := io.Copy(writer, file); err != nil {
		return "", false, fmt.Errorf("failed to write file to zip: %v", err)
	}

	// 关闭所有writer
	if err := archive.Close(); err != nil {
		return "", false, fmt.Errorf("failed to close zip archive: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		return "", false, fmt.Errorf("failed to close zip file: %v", err)
	}

	// 检查压缩效果
	compressedInfo, err := os.Stat(zipFile)
	if err != nil {
		return "", false, fmt.Errorf("failed to get compressed file info: %v", err)
	}
	compressedSize := compressedInfo.Size()

	// 计算压缩率
	compressionRatio := float64(compressedSize) / float64(originalSize)
	fmt.Printf("Original size: %.2f MB\n", float64(originalSize)/1024/1024)
	fmt.Printf("Compressed size: %.2f MB\n", float64(compressedSize)/1024/1024)
	fmt.Printf("Compression ratio: %.2f%%\n", compressionRatio*100)

	// 如果压缩后文件更大或者压缩率大于95%，则使用原文件
	if compressionRatio >= 0.95 {
		fmt.Println("Compression not effective, using original file")
		os.Remove(zipFile) // 删除无效的压缩文件
		return inputFile, false, nil
	}

	return zipFile, true, nil
}

// UploadFile 上传单个文件到Alist
func (u *FileUploader) UploadFile(srcPath, destPath string) (map[string]interface{}, error) {
	// 如果没有token，先获取token
	if u.token == "" {
		if err := u.getAlistToken(); err != nil {
			return nil, fmt.Errorf("failed to get Alist token: %v", err)
		}
	}

	// 打开要上传的文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}

	// 创建multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	part, err := writer.CreateFormFile("file", filepath.Base(srcPath))
	if err != nil {
		srcFile.Close()
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	if _, err := io.Copy(part, srcFile); err != nil {
		srcFile.Close()
		return nil, fmt.Errorf("failed to copy file content: %v", err)
	}

	// 关闭源文件
	srcFile.Close()

	// 添加路径参数，确保路径以斜杠开头
	filePath := filepath.Join(u.config.AlistPath, time.Now().Format("20060102"), filepath.Base(srcPath))
	filePath = "/" + strings.TrimPrefix(strings.ReplaceAll(filePath, "\\", "/"), "/")

	// 将路径中的斜杠替换为 %2F
	encodedPath := strings.ReplaceAll(filePath, "/", "%2F")

	if err := writer.WriteField("path", filePath); err != nil {
		return nil, fmt.Errorf("failed to write path field: %v", err)
	}

	// 关闭writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("PUT", u.config.AlistURL+"/api/fs/form", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", u.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Referer", u.config.AlistURL+u.config.AlistPath)
	req.Header.Set("file-path", encodedPath)

	// 打印请求头信息
	fmt.Println("\nRequest Headers:")
	fmt.Printf("Authorization: %s\n", u.token)
	fmt.Printf("Content-Type: %s\n", writer.FormDataContentType())
	fmt.Printf("Referer: %s\n", u.config.AlistURL+u.config.AlistPath)
	fmt.Printf("file-path: %s\n", encodedPath)
	fmt.Printf("Request URL: %s\n", req.URL.String())
	fmt.Printf("Request Method: %s\n", req.Method)
	fmt.Printf("Upload Path: %s\n\n", filePath)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// 检查响应状态
	if code, ok := result["code"].(float64); !ok || code != 200 {
		// 如果是token过期，尝试重新获取token并重试
		if code == 401 {
			u.token = "" // 清除旧token
			if err := u.getAlistToken(); err != nil {
				return nil, fmt.Errorf("failed to refresh token: %v", err)
			}
			// 重试上传
			return u.UploadFile(srcPath, destPath)
		}
		return nil, fmt.Errorf("upload failed: %v", result["message"])
	}

	// 上传成功后，等待一小段时间确保文件句柄完全释放
	time.Sleep(100 * time.Millisecond)

	// 删除源文件
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := os.Remove(srcPath); err != nil {
			if i < maxRetries-1 {
				log.Printf("Attempt %d: Failed to remove source file %s: %v, retrying...", i+1, srcPath, err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			log.Printf("Warning: failed to remove source file %s after %d attempts: %v", srcPath, maxRetries, err)
		} else {
			log.Printf("Successfully removed source file: %s", srcPath)
			break
		}
	}

	return result, nil
}

// UploadMergedFiles 上传合并后的文件
func (u *FileUploader) UploadMergedFiles(outputDir string) error {
	// 获取所有合并后的文件
	files, err := filepath.Glob(filepath.Join(outputDir, u.config.FilePattern))
	if err != nil {
		return fmt.Errorf("failed to list files: %v", err)
	}

	for _, file := range files {
		// 检查文件年龄
		fileInfo, err := os.Stat(file)
		if err != nil {
			log.Printf("Warning: failed to get file info for %s: %v", file, err)
			continue
		}

		// 如果文件超过最大保留天数，则删除
		if u.config.MaxFileAge > 0 {
			age := time.Since(fileInfo.ModTime())
			if age > time.Duration(u.config.MaxFileAge)*24*time.Hour {
				if err := os.Remove(file); err != nil {
					log.Printf("Warning: failed to remove old file %s: %v", file, err)
				} else {
					log.Printf("Removed old file: %s", file)
				}
				continue
			}
		}

		// 构建目标路径
		fileName := filepath.Base(file)
		destPath := filepath.Join(u.config.AlistPath, fileName)

		// 尝试上传文件，最多重试指定次数
		var uploadErr error
		for i := 0; i < u.config.RetryCount; i++ {
			if _, err := u.UploadFile(file, destPath); err != nil {
				uploadErr = err
				log.Printf("Upload attempt %d/%d failed for %s: %v", i+1, u.config.RetryCount, file, err)
				time.Sleep(time.Duration(u.config.RetryDelay) * time.Second)
				continue
			}
			log.Printf("Successfully uploaded file to Alist: %s", file)
			uploadErr = nil
			break
		}

		if uploadErr != nil {
			log.Printf("Failed to upload file %s after %d attempts: %v", file, u.config.RetryCount, uploadErr)
		}
	}

	return nil
}

// CleanupOldFiles 清理旧文件
func (u *FileUploader) CleanupOldFiles(outputDir string) error {
	if u.config.MaxFileAge <= 0 {
		return nil
	}

	files, err := filepath.Glob(filepath.Join(outputDir, u.config.FilePattern))
	if err != nil {
		return fmt.Errorf("failed to list files: %v", err)
	}

	now := time.Now()
	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			log.Printf("Warning: failed to get file info for %s: %v", file, err)
			continue
		}

		age := now.Sub(fileInfo.ModTime())
		if age > time.Duration(u.config.MaxFileAge)*24*time.Hour {
			if err := os.Remove(file); err != nil {
				log.Printf("Warning: failed to remove old file %s: %v", file, err)
			} else {
				log.Printf("Removed old file: %s", file)
			}
		}
	}

	return nil
}
