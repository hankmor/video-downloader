package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/hankmor/vdd/core/logger"
	"github.com/nfnt/resize"
)

// compressImage 压缩图片到指定最大尺寸（保持宽高比）
// 支持格式: webp, png, jpg, jpeg
// maxSize: 最大宽度或高度（像素）
func CompressImage(data []byte, maxSize uint) ([]byte, error) {
	// 尝试多种方式解码图片
	img, format, err := DecodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	logger.Infof("[缩略图] 成功解码图片，格式: %s, 尺寸: %dx%d", format, img.Bounds().Dx(), img.Bounds().Dy())

	// 获取原始尺寸
	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())

	// 缩放图片（如果需要）
	var resizedImg image.Image
	if width <= maxSize && height <= maxSize {
		// 图片已经足够小，不需要缩放，但后续会统一转换为JPEG以减小文件大小
		resizedImg = img
		logger.Infof("[缩略图] 图片尺寸已符合要求，无需缩放")
	} else {
		// 计算新尺寸（保持宽高比，最大边不超过maxSize）
		var newWidth, newHeight uint
		if width > height {
			newWidth = maxSize
			newHeight = uint(float64(height) * float64(maxSize) / float64(width))
		} else {
			newHeight = maxSize
			newWidth = uint(float64(width) * float64(maxSize) / float64(height))
		}

		// 使用 Lanczos3 算法进行高质量缩放
		resizedImg = resize.Thumbnail(newWidth, newHeight, img, resize.Lanczos3)
		logger.Infof("[缩略图] 图片已缩放: %dx%d -> %dx%d", width, height, newWidth, newHeight)
	}

	// 统一编码为JPEG格式（压缩率更高，文件更小）
	// JPEG格式对缩略图来说已经足够，且文件大小更小
	return EncodeAsJPEG(resizedImg, 85)
}

// decodeImage 尝试多种方式解码图片
// 支持格式: webp, png, jpeg/jpg
func DecodeImage(data []byte) (image.Image, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("图片数据为空")
	}

	reader := bytes.NewReader(data)

	// 方法1: 使用 image.Decode 自动识别格式
	// 这会尝试所有已注册的解码器（包括 webp, png, jpeg）
	img, format, err := image.Decode(reader)
	if err == nil && format != "" {
		// 标准化格式名称
		format = NormalizeFormat(format)
		logger.Infof("[缩略图] 自动识别格式成功: %s", format)
		return img, format, nil
	}

	// 方法2: 如果自动解码失败，尝试根据文件头手动识别
	detectedFormat := DetectImageFormat(data)
	if detectedFormat == "" {
		return nil, "", fmt.Errorf("无法识别图片格式，自动解码失败: %v", err)
	}

	logger.Infof("[缩略图] 通过文件头检测到格式: %s，重新尝试解码", detectedFormat)

	// 根据检测到的格式重新尝试解码
	reader.Seek(0, io.SeekStart)
	img, detectedFormat2, err := image.Decode(reader)
	if err != nil {
		return nil, "", fmt.Errorf("解码 %s 格式失败: %w", detectedFormat, err)
	}

	// 使用检测到的格式（优先使用 image.Decode 返回的格式，否则使用文件头检测的格式）
	if detectedFormat2 != "" {
		format = NormalizeFormat(detectedFormat2)
	} else {
		format = NormalizeFormat(detectedFormat)
	}

	return img, format, nil
}

// detectImageFormat 根据文件头检测图片格式
func DetectImageFormat(data []byte) string {
	if len(data) < 12 {
		return ""
	}

	// WebP: RIFF...WEBP
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "png"
	}

	// JPEG: FF D8 FF
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg"
	}

	return ""
}

// normalizeFormat 标准化格式名称
func NormalizeFormat(format string) string {
	format = strings.ToLower(format)
	switch format {
	case "jpeg", "jpg":
		return "jpeg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	default:
		return format
	}
}

func EncodeAsJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	jpegOptions := &jpeg.Options{
		Quality: quality, // 质量设置，范围 1-100
	}

	if err := jpeg.Encode(&buf, img, jpegOptions); err != nil {
		// JPEG编码失败，尝试PNG作为降级方案
		logger.Errorf("[缩略图] JPEG编码失败，尝试PNG: %v", err.Error())
		return EncodeAsPNG(img)
	}

	return buf.Bytes(), nil
}

func EncodeAsPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("PNG编码失败: %w", err)
	}
	return buf.Bytes(), nil
}
