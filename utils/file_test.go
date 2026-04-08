package utils

import (
	"fmt"
	"testing"
)

func TestResolveActualFilePath(t *testing.T) {
	f1 := "%(title)s-%(uploader)s.%(ext)s"
	f2 := "%(uploader)s-%(title)s.%(ext)s"
	f3 := "%(upload_date)s-%(title)s.%(ext)s"
	f4 := "%(title)s.%(ext)s"

	title := "This is a test title"
	uploader := "This is a test uploader"
	extension := "mp4"
	formatID := "1234567890"
	downloadDir := "/Users/hankmor/Downloads"
	uploadDate := "20250101"
	// Case 1: 假设启用了 ffmpeg 且合并 (isMergeToMp4=true)
	actualPath := EstimateActualFilePath(f1, title, extension, formatID, uploader, uploadDate, downloadDir, true)
	fmt.Printf("Case 1 (Merge=True): %s\n", actualPath)

	// Case 2: 假设未启用 ffmpeg (isMergeToMp4=false)
	// 预期后缀保持原样 (mp4)，但如果是其他后缀就不会被强转
	actualPath = EstimateActualFilePath(f2, title, extension, formatID, uploader, uploadDate, downloadDir, false)
	fmt.Printf("Case 2 (Merge=False): %s\n", actualPath)

	// Case 3: 包含不支持的占位符 (isMergeToMp4=true)
	actualPath = EstimateActualFilePath(f3, title, extension, formatID, uploader, uploadDate, downloadDir, true)
	fmt.Printf("Case 3 (Unsupported Placeholder): %s\n", actualPath)

	// Case 4: 简单格式 (isMergeToMp4=true)
	actualPath = EstimateActualFilePath(f4, title, extension, formatID, uploader, uploadDate, downloadDir, true)
	fmt.Printf("Case 4 (Simple): %s\n", actualPath)

	// Case 5: FormatID 含 +, Extension=webm, isMergeToMp4=true -> 应该变 mp4
	titleWebm := "Webm Video"
	formatIDPlus := "137+140"
	extWebm := "webm"
	pathMerged := EstimateActualFilePath(f4, titleWebm, extWebm, formatIDPlus, uploader, uploadDate, downloadDir, true)
	fmt.Printf("Case 5 (Format+, Webm->Mp4): %s\n", pathMerged)

	// Case 6: FormatID 含 +, Extension=webm, isMergeToMp4=false -> 应该保持 webm
	pathNoMerge := EstimateActualFilePath(f4, titleWebm, extWebm, formatIDPlus, uploader, uploadDate, downloadDir, false)
	fmt.Printf("Case 6 (Format+, Webm->Webm): %s\n", pathNoMerge)

}

func TestResolveActualFilePathAfterDownload(t *testing.T) {
	templatePath := "%(title)s-%(uploader)s.%(ext)s"
	title := "Rick Astley - Never Gonna Give You Up"
	downloadDir := "/Users/hankmor/Downloads"
	actualPath := ResolveActualFilePathAfterDownload(templatePath, title, downloadDir)
	fmt.Println(actualPath)
}
