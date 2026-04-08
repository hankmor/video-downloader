package utils

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hankmor/vdd/core/logger"
)

// EstimateActualFilePath 根据模板和元数据计算预期的文件路径
// 这是一个纯字符串处理函数，不管文件是否存在
// extension: 原始后缀
// isMergeToMp4: 是否启用了合并转 mp4 (通常取决于是否配置了 ffmpeg)
func EstimateActualFilePath(templatePath, title, extension, formatID, uploader, uploadDate, downloadDir string, isMergeToMp4 bool) string {
	if templatePath == "" {
		return ""
	}

	// 1. 确定基准目录
	dir := filepath.Dir(templatePath)
	filenameTemplate := filepath.Base(templatePath)

	// 如果模板路径本身没有目录部分，则使用 downloadDir
	if dir == "." || dir == "" {
		dir = downloadDir
	} else if !filepath.IsAbs(dir) {
		if downloadDir != "" {
			dir = filepath.Join(downloadDir, dir)
		}
	}

	// 2. 如果模板不包含占位符，直接返回组合后的绝对路径
	if !strings.Contains(filenameTemplate, "%(") {
		absDir, _ := filepath.Abs(dir)
		return filepath.Join(absDir, filenameTemplate)
	}

	// 3. 替换占位符
	cleanTitle := SanitizeFileName(title)
	cleanUploader := SanitizeFileName(uploader)

	filename := strings.ReplaceAll(filenameTemplate, "%(title)s", cleanTitle)
	filename = strings.ReplaceAll(filename, "%(uploader)s", cleanUploader)
	filename = strings.ReplaceAll(filename, "%(upload_date)s", uploadDate)
	filename = strings.ReplaceAll(filename, "%(id)s", formatID)

	// 处理后缀
	finalExt := extension

	// 1. 如果 formatID 包含 "+"，说明是 Video+Audio 合并格式
	// 并且启用了 merge 输出为 mp4 (即有 ffmpeg)
	if isMergeToMp4 && strings.Contains(formatID, "+") {
		finalExt = "mp4"
	}

	// 2. 如果未指定，默认 mp4
	if finalExt == "" {
		finalExt = "mp4"
	}
	// 移除点号以便统一处理
	finalExt = strings.TrimPrefix(finalExt, ".")

	filename = strings.ReplaceAll(filename, "%(ext)s", finalExt)

	// 4. 返回绝对路径
	absDir, _ := filepath.Abs(dir)
	return filepath.Join(absDir, filename)
}

// ResolveActualFilePathAfterDownload 解析实际文件路径
// yt-dlp 会处理文件名中的特殊字符，实际文件名可能与模板不完全一致
// 此函数尝试从模板路径和标题中推断实际文件路径
func ResolveActualFilePathAfterDownload(templatePath, title, downloadDir string) string {
	if templatePath == "" {
		return ""
	}

	// 如果路径不包含模板语法，直接返回
	if !strings.Contains(templatePath, "%(") {
		// 检查文件是否存在
		if FileExists(templatePath) {
			return templatePath
		}
		// 如果不存在，尝试规范化路径
		absPath, err := filepath.Abs(templatePath)
		if err == nil && FileExists(absPath) {
			return absPath
		}
		return ""
	}

	// 提取目录部分
	dir := filepath.Dir(templatePath)
	if dir == "" || dir == "." {
		dir = downloadDir
		if dir == "" {
			dir = GetDownloadDir()
		}
	}

	// 规范化目录路径
	absDir, err := filepath.Abs(dir)
	if err != nil {
		logger.Errorf("获取绝对路径失败: %v", err)
		absDir = dir
	}

	// 清理标题中的特殊字符（yt-dlp 会做类似处理）
	cleanTitle := SanitizeFileName(title)

	// 尝试查找文件：根据标题和常见扩展名
	extensions := []string{".mp4", ".mkv", ".webm", ".m4a", ".mp3"}
	for _, ext := range extensions {
		// 尝试精确匹配
		candidate := filepath.Join(absDir, cleanTitle+ext)
		if FileExists(candidate) {
			return candidate
		}

		// 尝试部分匹配（标题可能被截断或修改）
		// 查找目录中最近创建的、包含标题关键词的文件
		files, err := os.ReadDir(absDir)
		if err != nil {
			continue
		}

		// 查找最近修改的文件，文件名包含标题的一部分
		var bestMatch string
		var bestModTime time.Time
		titleWords := strings.Fields(strings.ToLower(cleanTitle))
		if len(titleWords) > 0 {
			firstWord := titleWords[0]
			if len(firstWord) > 3 { // 至少3个字符才匹配
				for _, file := range files {
					if file.IsDir() {
						continue
					}
					fileName := strings.ToLower(file.Name())
					if strings.Contains(fileName, firstWord) && strings.HasSuffix(fileName, ext) {
						info, err := file.Info()
						if err != nil {
							continue
						}
						if info.ModTime().After(bestModTime) {
							bestModTime = info.ModTime()
							bestMatch = filepath.Join(absDir, file.Name())
						}
					}
				}
			}
		}

		if bestMatch != "" {
			return bestMatch
		}
	}

	return ""
}

// SanitizeFileName 清理文件名，移除或替换特殊字符
func SanitizeFileName(name string) string {
	// 移除或替换常见的文件系统不支持的字符
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\n", " ",
		"\r", " ",
		"\t", " ",
	)
	cleaned := replacer.Replace(name)

	// 移除首尾空格和多余空格
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	// 限制长度（避免文件名过长）
	if len(cleaned) > 200 {
		cleaned = cleaned[:200]
	}

	return cleaned
}
