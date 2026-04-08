package utils

import (
	"crypto/md5"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

//go:embed all:bundled
var embeddedBinaries embed.FS

var (
	extractOnce  sync.Once
	extractedDir string
)

// GetYtDlpPath 获取 yt-dlp 路径
func GetYtDlpPath() string {
	// 确保二进制文件已提取
	extractOnce.Do(extractEmbeddedBinaries)

	ytdlpName := "yt-dlp"
	if runtime.GOOS == "windows" {
		ytdlpName = "yt-dlp.exe"
	}

	// 优先使用提取的嵌入版本
	if extractedDir != "" {
		embeddedPath := filepath.Join(extractedDir, ytdlpName)
		if fileExists(embeddedPath) {
			return embeddedPath
		}
	}

	// 降级方案 1：检查开发环境的 assets/bundled/
	projectRoot := getProjectRoot()
	devPath := filepath.Join(projectRoot, "assets", "bundled", ytdlpName)
	if fileExists(devPath) {
		return devPath
	}

	// 降级方案 2：使用系统安装的 yt-dlp
	return ytdlpName
}

// extractEmbeddedBinaries 提取嵌入的二进制文件到临时目录
func extractEmbeddedBinaries() {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "vdd-*")
	if err != nil {
		return
	}

	extractedDir = tmpDir

	// 提取 yt-dlp
	extractBinary("yt-dlp")

	// 提取 ffmpeg
	extractBinary("ffmpeg")
}

// extractBinary 提取单个二进制文件
func extractBinary(name string) {
	if extractedDir == "" {
		return
	}

	// 根据平台添加扩展名
	fileName := name
	if runtime.GOOS == "windows" {
		fileName = name + ".exe"
	}

	// 读取嵌入的文件
	embeddedPath := "bundled/" + fileName
	data, err := embeddedBinaries.ReadFile(embeddedPath)
	if err != nil {
		// 嵌入的文件不存在（可能是交叉编译或开发环境）
		return
	}

	// 写入临时文件
	extractedPath := filepath.Join(extractedDir, fileName)
	if err := os.WriteFile(extractedPath, data, 0755); err != nil {
		return
	}
}

// GetFFmpegPath 获取 ffmpeg 路径
func GetFFmpegPath() string {
	// 确保二进制文件已提取
	extractOnce.Do(extractEmbeddedBinaries)

	ffmpegName := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegName = "ffmpeg.exe"
	}

	// 优先使用提取的嵌入版本
	if extractedDir != "" {
		embeddedPath := filepath.Join(extractedDir, ffmpegName)
		if fileExists(embeddedPath) {
			return embeddedPath
		}
	}

	// 降级方案 1：检查开发环境的 utils/bundled/
	projectRoot := getProjectRoot()
	devPath := filepath.Join(projectRoot, "utils", "bundled", ffmpegName)
	if fileExists(devPath) {
		return devPath
	}

	// 降级方案 2：使用系统安装的 ffmpeg
	return ffmpegName
}

// GetConfigDir 获取配置目录
func GetConfigDir() string {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".vdd")

	// 确保目录存在
	os.MkdirAll(configDir, 0755)

	return configDir
}

// GetDownloadDir 获取默认下载目录
func GetDownloadDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Downloads")
}

// fileExists 检查文件是否存在（内部使用）
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FileExists 检查文件是否存在（导出函数）
func FileExists(path string) bool {
	return fileExists(path)
}

// getProjectRoot 获取项目根目录（仅用于开发环境）
func getProjectRoot() string {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)

	// 向上查找 go.mod 文件
	dir := execDir
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return execDir
}

// OpenFolder 在文件管理器中打开文件或目录
func OpenFolder(path string) {
	var cmd *exec.Cmd

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		// 如果路径不存在 (比如文件被删除了)，尝试打开其父目录
		parent := filepath.Dir(path)
		if _, err := os.Stat(parent); err == nil {
			path = parent
			info, _ = os.Stat(path) // update info to directory
		} else {
			fmt.Printf("Cannot open path: %s (and parent not found)\n", path)
			return
		}
	}

	// 根据不同平台构建命令
	switch runtime.GOOS {
	case "windows":
		if !info.IsDir() {
			// 如果是文件，使用 /select, 选中它
			cmd = exec.Command("explorer", "/select,", path)
		} else {
			// 如果是目录，直接打开
			cmd = exec.Command("explorer", path)
		}
	case "darwin":
		if !info.IsDir() {
			// 如果是文件，使用 -R 在 Finder 中显示
			cmd = exec.Command("open", "-R", path)
		} else {
			// 如果是目录，直接打开
			cmd = exec.Command("open", path)
		}
	case "linux":
		// Linux 通常只能打开目录
		target := path
		if !info.IsDir() {
			target = filepath.Dir(path)
		}
		cmd = exec.Command("xdg-open", target)
	default:
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to open folder: %v\n", err)
	}
}

// GetFileSize 获取文件大小 (字节)
func GetFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// GetThumbnailCacheDir 获取缩略图缓存目录
func GetThumbnailCacheDir() string {
	configDir := GetConfigDir()
	cacheDir := filepath.Join(configDir, "thumbnails")
	
	// 确保目录存在
	os.MkdirAll(cacheDir, 0755)
	
	return cacheDir
}

// GetThumbnailCachePath 根据URL生成缓存文件路径
func GetThumbnailCachePath(imageURL string) string {
	// 使用MD5哈希URL生成文件名
	hash := md5.Sum([]byte(imageURL))
	filename := hex.EncodeToString(hash[:]) + ".jpg"
	return filepath.Join(GetThumbnailCacheDir(), filename)
}

// DeleteThumbnailCache 删除指定的缩略图缓存文件
func DeleteThumbnailCache(imageURL string) error {
	if imageURL == "" {
		return nil
	}
	cachePath := GetThumbnailCachePath(imageURL)
	if fileExists(cachePath) {
		return os.Remove(cachePath)
	}
	return nil
}
